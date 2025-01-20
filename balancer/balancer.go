// Copyright 2024 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package balancer

import (
	"fmt"
	"math"
	"slices"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
	"golang.org/x/exp/rand"
)

// Balancer is used to redistribute stream and consumer leaders in a cluster.
// The Balancer will first find all the leaders and peers for the given set of
// streams or consumers, and then determine an even distribution. If any of the
// servers is the leader for more than the even distribution, the balancer will
// step down a number of streams/consumers until the even distribution is met.
// Which streams/consumers are stepped down is determined randomly. We use
// preferred placement to move the leadership to a server with less than
// the even distribution on the same cluster. If stepping down fails, we
// will move on the next randomly selected server. If we get a second, similar
// failure the Balancer will return an error.
type Balancer struct {
	nc  *nats.Conn
	log api.Logger
}

type balanceEntity interface {
	LeaderStepDown(...*api.Placement) error
	Name() string
	ClusterInfo() (api.ClusterInfo, error)
}

type peer struct {
	name     string
	entities []balanceEntity
	offset   int
	clusters map[string]bool
}

// New returns a new instance of the Balancer
func New(nc *nats.Conn, log api.Logger) (*Balancer, error) {
	mgr, err := jsm.New(nc)
	if err != nil {
		return nil, err
	}

	apiLvl, err := mgr.MetaApiLevel(true)
	if err != nil {
		return nil, err
	}

	if apiLvl < 1 {
		return nil, fmt.Errorf("invalid server version. Balancer requires server version 2.11.0 or higher")
	}

	return &Balancer{
		nc:  nc,
		log: log,
	}, nil
}

func (b *Balancer) calcOffset(servers *map[string]*peer, evenDistribution int) {
	for _, s := range *servers {
		s.offset = len(s.entities) - evenDistribution
	}
}

// We consider a balanced stream to be one with an offset of 0 or less
func (b *Balancer) isBalanced(servers map[string]*peer) bool {
	for _, s := range servers {
		if s.offset > 0 {
			return false
		}
	}

	return true
}

func (b *Balancer) mapEntityToServers(entity balanceEntity, serverMap map[string]*peer) (map[string]*peer, error) {
	info, err := entity.ClusterInfo()
	if err != nil {
		return nil, err
	}

	leaderName := info.Leader
	_, ok := serverMap[leaderName]
	if !ok {
		tmp := peer{
			name:     leaderName,
			entities: []balanceEntity{},
			clusters: map[string]bool{},
		}
		serverMap[leaderName] = &tmp
	}
	serverMap[leaderName].entities = append(serverMap[leaderName].entities, entity)
	serverMap[leaderName].clusters[info.Name] = true

	for _, replica := range info.Replicas {
		_, ok = serverMap[replica.Name]
		if !ok {
			tmp := &peer{
				name:     replica.Name,
				entities: []balanceEntity{},
				clusters: map[string]bool{info.Name: true},
			}
			serverMap[replica.Name] = tmp
		}
	}

	return serverMap, nil
}

func (b *Balancer) calcDistribution(entities, servers int) int {
	evenDistributionf := float64(entities) / float64(servers)
	if evenDistributionf == float64(int64(evenDistributionf)) {
		return int(evenDistributionf)
	}
	return int(math.Ceil(evenDistributionf + 0.5))
}

func (b *Balancer) balance(servers map[string]*peer, evenDistribution int, typeHint string) (int, error) {
	steppedDown := 0
	for !b.isBalanced(servers) {
		for _, s := range servers {
			if s.offset > 0 {
				b.log.Infof("Found server '%s' with offset of %d. Rebalancing", s.name, s.offset)
				retries := 0
				for s.offset > 0 {
					// find a random stream (or consumer) to move to another server
					randomIndex := rand.Intn(len(s.entities))
					entity := s.entities[randomIndex]
					clusterinfo, err := entity.ClusterInfo()
					if err != nil {
						return 0, fmt.Errorf("unable to get clusterinfo for %s '%s'. %s", typeHint, entity.Name(), err)
					}

					for _, ns := range servers {
						if ns.offset < 0 && ns.clusters[clusterinfo.Name] {
							b.log.Infof("Requesting leader '%s' step down for %s '%s'. New preferred leader is %s.", s.name, typeHint, entity.Name(), ns.name)
							placement := api.Placement{Preferred: ns.name, Cluster: clusterinfo.Name}
							err := entity.LeaderStepDown(&placement)
							if err != nil {
								b.log.Errorf("Unable to step down leader for  %s - %s", entity.Name(), err)
								// If we failed to step down the stream, decrement the iterator so that we don't kick one too few
								// Limit this to one retry, if we can't step down multiple leaders something is wrong
								if retries == 0 {
									retries++
									s.entities = slices.Delete(s.entities, randomIndex, randomIndex+1)
									break
								}
								return 0, err
							}
							b.log.Infof("Successful step down for %s '%s'", typeHint, entity.Name())
							retries = 0
							steppedDown += 1
							ns.offset += 1
							s.offset -= 1
							// Remove the entity we just moved from the server so it can't be randomly selected again
							s.entities = slices.Delete(s.entities, randomIndex, randomIndex+1)
							break
						}
					}
				}
			}
		}
		// We recalculate the offset count, we can't be 100% sure entities moved to their preferred server
		b.calcOffset(&servers, evenDistribution)
	}

	return steppedDown, nil
}

// BalanceStreams finds the expected distribution of stream leaders over servers
// and forces leader election on any with an uneven distribution
func (b *Balancer) BalanceStreams(streams []*jsm.Stream) (int, error) {
	var err error
	servers := map[string]*peer{}

	for _, s := range streams {
		servers, err = b.mapEntityToServers(s, servers)
		if err != nil {
			return 0, err
		}
	}

	b.log.Debugf("found %d streams on %d servers\n", len(streams), len(servers))
	evenDistribution := b.calcDistribution(len(streams), len(servers))
	b.log.Debugf("even distribution is %d\n", evenDistribution)
	b.log.Debugf("calculating offset for each server")
	b.calcOffset(&servers, evenDistribution)

	return b.balance(servers, evenDistribution, "stream")
}

// BalanceConsumers finds the expected distribution of consumer leaders over servers
// and forces leader election on any with an uneven distribution
func (b *Balancer) BalanceConsumers(consumers []*jsm.Consumer) (int, error) {
	var err error
	servers := map[string]*peer{}

	for _, s := range consumers {
		servers, err = b.mapEntityToServers(s, servers)
		if err != nil {
			return 0, err
		}
	}

	b.log.Debugf("found %d consumers on %d servers\n", len(consumers), len(servers))
	evenDistribution := b.calcDistribution(len(consumers), len(servers))
	b.log.Debugf("even distribution is %d\n", evenDistribution)
	b.log.Debugf("calculating offset for each server")
	b.calcOffset(&servers, evenDistribution)

	return b.balance(servers, evenDistribution, "consumer")
}
