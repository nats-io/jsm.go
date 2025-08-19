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

package jsm

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/jsm.go/api"
)

type consumerMatcher func([]*Consumer) ([]*Consumer, error)

func truePtr() *bool {
	t := true
	return &t
}

type consumerQuery struct {
	replicas          int
	expression        string
	matchers          []consumerMatcher
	invert            bool
	isPull            *bool
	isPush            *bool
	isBound           *bool
	isPinned          *bool
	waiting           int
	ackPending        int
	pending           uint64
	leader            string
	apiLevel          int
	ageLimit          time.Duration
	lastDeliveryLimit time.Duration
}

type ConsumerQueryOpt func(query *consumerQuery) error

// ConsumerQueryApiLevelMin limits results to assets requiring API Level above or equal to level
func ConsumerQueryApiLevelMin(level int) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.apiLevel = level
		return nil
	}
}

// ConsumerQueryLeaderServer finds clustered consumers where a certain node is the leader
func ConsumerQueryLeaderServer(server string) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.leader = server
		return nil
	}
}

// ConsumerQueryIsPull finds only Pull consumers
func ConsumerQueryIsPull() ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.isPull = truePtr()
		return nil
	}
}

// ConsumerQueryIsPush finds only Push consumers
func ConsumerQueryIsPush() ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.isPush = truePtr()
		return nil
	}
}

// ConsumerQueryIsPinned finds consumers with pinned clients on all their groups
func ConsumerQueryIsPinned() ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.isPinned = truePtr()
		return nil
	}
}

// ConsumerQueryIsBound finds push consumers that are bound or pull consumers with waiting pulls
func ConsumerQueryIsBound() ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.isBound = truePtr()
		return nil
	}
}

// ConsumerQueryWithFewerWaiting finds consumers with fewer waiting pulls
func ConsumerQueryWithFewerWaiting(waiting int) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		if waiting <= 0 {
			return errors.New("waiting must be greater than zero")
		}

		q.waiting = waiting
		return nil
	}
}

// ConsumerQueryWithFewerAckPending finds consumers with fewer pending messages
func ConsumerQueryWithFewerAckPending(pending int) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		if pending <= 0 {
			return errors.New("pending must be greater than zero")
		}

		q.ackPending = pending
		return nil
	}
}

// ConsumerQueryWithFewerPending finds consumers with fewer unprocessed messages
func ConsumerQueryWithFewerPending(pending uint64) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.pending = pending
		return nil
	}
}

// ConsumerQueryOlderThan finds consumers older than age
func ConsumerQueryOlderThan(age time.Duration) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.ageLimit = age
		return nil
	}
}

// ConsumerQueryWithDeliverySince finds only consumers that has had deliveries since ts
func ConsumerQueryWithDeliverySince(age time.Duration) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.lastDeliveryLimit = age
		return nil
	}
}

// ConsumerQueryReplicas finds streams with a certain number of replicas or less
func ConsumerQueryReplicas(r uint) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.replicas = int(r)
		return nil
	}
}

// ConsumerQueryInvert inverts the logic of filters, older than becomes newer than and so forth
func ConsumerQueryInvert() ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.invert = true
		return nil
	}
}

// QueryConsumers filters the streams found in JetStream using various filter options
func (s *Stream) QueryConsumers(opts ...ConsumerQueryOpt) ([]*Consumer, error) {
	q := &consumerQuery{}
	for _, opt := range opts {
		err := opt(q)
		if err != nil {
			return nil, err
		}
	}

	if q.isPull != nil && q.isPush != nil {
		return nil, fmt.Errorf("cannot match pull and push concurrently")
	}

	q.matchers = []consumerMatcher{
		q.matchExpression,
		q.matchPull,
		q.matchPush,
		q.matchBound,
		q.matchPinned,
		q.matchAckPending,
		q.matchWaiting,
		q.matchPending,
		q.matchAge,
		q.matchDelivery,
		q.matchReplicas,
		q.matchLeaderServer,
		q.matchApiLevel,
	}

	var consumers []*Consumer
	_, _, err := s.EachConsumer(func(c *Consumer) {
		consumers = append(consumers, c)
	})
	if err != nil {
		return nil, err
	}

	return q.Filter(consumers)
}

func (q *consumerQuery) Filter(consumers []*Consumer) ([]*Consumer, error) {
	matched := consumers[:]
	var err error

	for _, matcher := range q.matchers {
		matched, err = matcher(matched)
		if err != nil {
			return nil, err
		}
	}

	return matched, nil
}

func (q *consumerQuery) cbMatcher(consumers []*Consumer, onlyIf bool, cb func(*Consumer) bool) ([]*Consumer, error) {
	if !onlyIf {
		return consumers, nil
	}

	var matched []*Consumer

	for _, consumer := range consumers {
		if cb(consumer) {
			matched = append(matched, consumer)
		}
	}

	return matched, nil
}

func (q *consumerQuery) matchLeaderServer(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.leader != "", func(consumer *Consumer) bool {
		nfo, _ := consumer.LatestState()
		if nfo.Cluster == nil {
			return false
		}

		return (!q.invert && nfo.Cluster.Leader == q.leader) || (q.invert && nfo.Cluster.Leader != q.leader)
	})
}

func (q *consumerQuery) matchReplicas(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.replicas > 0, func(consumer *Consumer) bool {
		return (!q.invert && consumer.Replicas() <= q.replicas) || (q.invert && consumer.Replicas() >= q.replicas)
	})
}

func (q *consumerQuery) matchDelivery(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.lastDeliveryLimit > 0, func(consumer *Consumer) bool {
		nfo, _ := consumer.LatestState()

		return nfo.Delivered.Last != nil && (!q.invert && time.Since(*nfo.Delivered.Last) < q.lastDeliveryLimit) || (q.invert && time.Since(nfo.Created) > q.ageLimit)
	})
}

func (q *consumerQuery) matchAge(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.ageLimit > 0, func(consumer *Consumer) bool {
		nfo, _ := consumer.LatestState()

		return !nfo.Created.IsZero() && (!q.invert && time.Since(nfo.Created) < q.ageLimit) || (q.invert && time.Since(nfo.Created) > q.ageLimit)
	})
}

func (q *consumerQuery) matchPending(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.pending > 0, func(consumer *Consumer) bool {
		nfo, _ := consumer.LatestState()

		return nfo.NumPending > 0 && (!q.invert && nfo.NumPending < q.pending) || (q.invert && nfo.NumPending > q.pending)
	})
}

func (q *consumerQuery) matchAckPending(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.ackPending > 0, func(consumer *Consumer) bool {
		nfo, _ := consumer.LatestState()

		return nfo.NumAckPending > 0 && (!q.invert && nfo.NumAckPending < q.ackPending) || (q.invert && nfo.NumAckPending > q.ackPending)
	})
}

func (q *consumerQuery) matchWaiting(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.waiting > 0, func(consumer *Consumer) bool {
		nfo, _ := consumer.LatestState()

		return nfo.NumWaiting > 0 && (!q.invert && nfo.NumWaiting < q.waiting) || (q.invert && nfo.NumWaiting > q.waiting)
	})
}

func (q *consumerQuery) matchPinned(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.isPinned != nil, func(consumer *Consumer) bool {
		if !consumer.IsPinnedClientPriority() {
			return false
		}

		nfo, _ := consumer.LatestState()

		var pinned int
		for _, group := range nfo.PriorityGroups {
			if group.PinnedClientID != "" {
				pinned++
			}
		}
		isPinned := pinned == len(nfo.Config.PriorityGroups)

		return (!q.invert && isPinned) || (q.invert && !isPinned)
	})
}

func (q *consumerQuery) matchBound(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.isBound != nil, func(consumer *Consumer) bool {
		nfo, _ := consumer.LatestState()
		bound := nfo.PushBound || nfo.NumWaiting > 0

		return (!q.invert && bound) || (q.invert && !bound)
	})
}

func (q *consumerQuery) matchPush(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.isPush != nil, func(consumer *Consumer) bool {
		return (!q.invert && consumer.IsPushMode()) || (q.invert && !consumer.IsPushMode())
	})
}

func (q *consumerQuery) matchPull(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.isPull != nil, func(consumer *Consumer) bool {
		return (!q.invert && consumer.IsPullMode()) || (q.invert && !consumer.IsPullMode())
	})
}

func (q *consumerQuery) matchApiLevel(consumers []*Consumer) ([]*Consumer, error) {
	return q.cbMatcher(consumers, q.apiLevel > 0, func(consumer *Consumer) bool {
		var v string
		var requiredLevel int

		if len(consumer.Metadata()) > 0 {
			v = consumer.Metadata()[api.JsMetaRequiredServerLevel]
			if v != "" {
				requiredLevel, _ = strconv.Atoi(v)
			}
		}

		return (!q.invert && requiredLevel >= q.apiLevel) || (q.invert && requiredLevel < q.apiLevel)
	})
}
