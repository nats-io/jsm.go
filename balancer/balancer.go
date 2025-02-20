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
	"math/rand/v2"
	"slices"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
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
}

type cluster struct {
	name                 string
	peers                map[string]*peer
	balancedDistribution int
}

const BALANCERUNS = 5

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

func (b *Balancer) createClusterMappings(e balanceEntity, clusterMap map[string]*cluster) (map[string]*cluster, error) {
	info, err := e.ClusterInfo()
	if err != nil {
		return nil, err
	}
	clustername := info.Name
	_, ok := clusterMap[clustername]
	if !ok {
		tmp := cluster{
			name:                 clustername,
			peers:                map[string]*peer{},
			balancedDistribution: 0,
		}
		clusterMap[clustername] = &tmp
	}

	leadername := info.Leader
	active := clusterMap[clustername]

	// This can happen if there very outdated or broken streams in the cluster. Very edge case
	// but if we don't check for it really messes up the balancer math.
	if leadername == "" {
		b.log.Warnf("Found entity without leader %s", e.Name())
	} else {
		_, ok = active.peers[leadername]
		if !ok {
			tmp := peer{
				name:     leadername,
				entities: []balanceEntity{},
				offset:   0,
			}
			active.peers[leadername] = &tmp
		}
		active.peers[leadername].entities = append(active.peers[leadername].entities, e)
	}

	for _, replica := range info.Replicas {
		_, ok = active.peers[replica.Name]
		if !ok {
			tmp := &peer{
				name:     replica.Name,
				entities: []balanceEntity{},
				offset:   0,
			}
			active.peers[replica.Name] = tmp
		}
	}

	return clusterMap, nil
}

func (b *Balancer) balance(servers map[string]*peer, evenDistribution int, cluster string, typeHint string) (int, error) {
	steppedDown := 0
	for i := 1; i <= BALANCERUNS; i++ {
		for _, s := range servers {
			if s.offset > 0 {
				b.log.Infof("Rebalancing server '%s' with offset of %d", s.name, s.offset)
				retries := 0
				for s.offset > 0 {
					// find a random stream (or consumer) to move to another server
					randomIndex := rand.IntN(len(s.entities))
					entity := s.entities[randomIndex]

					b.log.Infof("Moving %s to available server in cluster %s", entity.Name(), cluster)
					for _, ns := range servers {
						if ns.offset < 0 {
							b.log.Infof("Requesting leader '%s' step down for %s '%s'. New preferred leader is %s.", s.name, typeHint, entity.Name(), ns.name)
							placement := api.Placement{Preferred: ns.name, Cluster: cluster}
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

func (b *Balancer) calcClusterDistribution(c *cluster) int {
	servercount := len(c.peers)
	entitycount := 0

	for _, e := range c.peers {
		entitycount += len(e.entities)
	}

	evenDistributionf := float64(entitycount) / float64(servercount)
	if evenDistributionf == float64(int64(evenDistributionf)) {
		return int(evenDistributionf)
	}
	return int(math.Ceil(evenDistributionf + 0.5))
}

func (b *Balancer) calcLeaderOffset(p map[string]*peer, distribution int) {
	for _, v := range p {
		v.offset = len(v.entities) - distribution
	}
}

func (b Balancer) logClusterStats(clusterMap map[string]*cluster) {
	for k, v := range clusterMap {
		b.log.Infof("Found cluster %s with a balanced distribution of %d", k, v.balancedDistribution)
		b.log.Infof("Cluster %s has %d available peers", k, len(v.peers))
		for _, server := range v.peers {
			b.log.Infof("Nats server '%s' is the lead of %d entities with a offset of %d", server.name, len(server.entities), server.offset)
		}
		b.log.Infof("")
	}
}

// BalanceStreams finds the expected distribution of stream leaders over servers
// and forces leader election on any with an uneven distribution
func (b *Balancer) BalanceStreams(streams []*jsm.Stream) (int, error) {
	var err error
	clusterMap := map[string]*cluster{}
	balanced := 0

	// Formulate our view of the world and all it's clusters
	b.log.Debugf("building relationship between clusters, leaders and streams")
	for _, s := range streams {
		clusterMap, err = b.createClusterMappings(s, clusterMap)
		if err != nil {
			return 0, err
		}
	}

	// Figure out what the balanced distribution for each cluster looks like
	for k, v := range clusterMap {
		b.log.Debugf("calculating balanced distribution for Nats cluster %s", k)
		v.balancedDistribution = b.calcClusterDistribution(v)

		b.log.Debugf("calculating offset for each server in Nats cluster %s", k)
		b.calcLeaderOffset(v.peers, v.balancedDistribution)
	}

	b.logClusterStats(clusterMap)
	// Balance each cluster
	for k, v := range clusterMap {
		b.log.Debugf("balancing streams on cluster %s", k)
		b, err := b.balance(v.peers, v.balancedDistribution, k, "stream")
		if err != nil {
			return 0, err
		}
		balanced += b
	}

	return balanced, nil
}

// BalanceConsumers finds the expected distribution of consumer leaders over servers
// and forces leader election on any with an uneven distribution
func (b *Balancer) BalanceConsumers(consumers []*jsm.Consumer) (int, error) {
	var err error
	clusterMap := map[string]*cluster{}
	balanced := 0

	// Formulate our view of the world and all it's clusters
	b.log.Debugf("building relationship between clusters, leaders and consumers")
	for _, s := range consumers {
		clusterMap, err = b.createClusterMappings(s, clusterMap)
		if err != nil {
			return 0, err
		}
	}

	// Figure out what the balanced distribution for each cluster looks like
	for k, v := range clusterMap {
		b.log.Debugf("calculating balanced distribution for Nats cluster %s", k)
		v.balancedDistribution = b.calcClusterDistribution(v)

		b.log.Debugf("calculating offset for each server in Nats cluster %s", k)
		b.calcLeaderOffset(v.peers, v.balancedDistribution)
	}

	b.logClusterStats(clusterMap)
	// Balance each cluster
	for k, v := range clusterMap {
		b.log.Debugf("balancing consumers on cluster %s", k)
		b, err := b.balance(v.peers, v.balancedDistribution, k, "consumer")
		if err != nil {
			return 0, err
		}
		balanced += b
	}

	return balanced, nil
}
