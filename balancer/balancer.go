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
// Which streams/consumers are stepped down is determined randomly.
// If stepping down fails, or if the same server is elected the leader again,
// we will move on the next randomly selected server. If we get a second, similar
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
	hostname    string
	entities    []balanceEntity
	leaderCount int
	rebalance   int
}

// New returns a new instance of the Balancer
func New(nc *nats.Conn, log api.Logger) (*Balancer, error) {
	return &Balancer{
		nc:  nc,
		log: log,
	}, nil
}

func (b *Balancer) updateServersWithExclude(servers map[string]*peer, exclude string) (map[string]*peer, error) {
	updated := map[string]*peer{}
	var err error

	for _, s := range servers {
		if s.hostname == exclude {
			continue
		}
		for _, e := range s.entities {
			updated, err = b.mapEntityToServers(e, updated)
			if err != nil {
				return updated, err
			}
		}
	}

	return updated, nil
}

func (b *Balancer) getOvers(server map[string]*peer, evenDistribution int) {
	for _, s := range server {
		if s.leaderCount == 0 {
			continue
		}

		if over := s.leaderCount - evenDistribution; over > 0 {
			s.rebalance = over
		}
	}
}

func (b *Balancer) isBalanced(servers map[string]*peer) bool {
	for _, s := range servers {
		if s.rebalance > 0 {
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
			hostname:    leaderName,
			entities:    []balanceEntity{},
			leaderCount: 0,
		}
		serverMap[leaderName] = &tmp
	}
	serverMap[leaderName].entities = append(serverMap[leaderName].entities, entity)
	serverMap[leaderName].leaderCount += 1

	for _, replica := range info.Replicas {
		_, ok = serverMap[replica.Name]
		if !ok {
			tmp := peer{
				hostname:    replica.Name,
				entities:    []balanceEntity{},
				leaderCount: 0,
			}
			serverMap[replica.Name] = &tmp
		}
	}

	return serverMap, nil
}

func (b *Balancer) calcDistribution(entities, servers int) int {
	evenDistributionf := float64(entities) / float64(servers)
	return int(math.Floor(evenDistributionf + 0.5))
}

func (b *Balancer) balance(servers map[string]*peer, evenDistribution int) (int, error) {
	var err error
	steppedDown := 0

	for !b.isBalanced(servers) {
		for _, s := range servers {
			// skip servers that aren't leaders
			if s.leaderCount == 0 {
				continue
			}

			if s.rebalance > 0 {
				b.log.Infof("Found server '%s' with %d entities over the even distribution\n", s.hostname, s.rebalance)
				// Now we have to kick a random selection of streams where number = rebalance
				retries := 0
				for i := 0; i < s.rebalance; i++ {
					randomIndex := rand.Intn(len(s.entities))
					entity := s.entities[randomIndex]
					if entity == nil {
						return steppedDown, fmt.Errorf("no more valid entities to balance")
					}
					b.log.Infof("Requesting leader (%s) step down for  %s", s.hostname, entity.Name())

					err = entity.LeaderStepDown()
					if err != nil {
						b.log.Errorf("Unable to step down leader for  %s - %s", entity.Name(), err)
						// If we failed to step down the stream, decrement the iterator so that we don't kick one too few
						// Limit this to one retry, if we can't step down multiple leaders something is wrong
						if retries == 0 {
							i--
							retries++
							s.entities = slices.Delete(s.entities, randomIndex, randomIndex+1)
							continue
						}
						return 0, err
					}

					b.log.Infof("Successful step down '%s'", entity.Name())
					retries = 0
					s.entities = slices.Delete(s.entities, randomIndex, randomIndex+1)
					steppedDown += 1
				}

				// finally, if we rebalanced a server we update the servers list and start again, excluding the one we just rebalanced
				servers, err = b.updateServersWithExclude(servers, s.hostname)
				if err != nil {
					return steppedDown, err
				}
				b.getOvers(servers, evenDistribution)
				break
			}
		}
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
	b.getOvers(servers, evenDistribution)

	return b.balance(servers, evenDistribution)
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
	b.getOvers(servers, evenDistribution)

	return b.balance(servers, evenDistribution)
}
