// Copyright 2022 The NATS Authors
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
	"regexp"
	"time"
)

type StreamQuery struct {
	Server         *regexp.Regexp
	Cluster        *regexp.Regexp
	ConsumersLimit *int
	Empty          *bool
	IdlePeriod     *time.Duration
	CreatedPeriod  *time.Duration
	Invert         bool
}

func (m *Manager) QueryStreams(q StreamQuery) ([]*Stream, error) {
	streams, err := m.Streams()
	if err != nil {
		return nil, err
	}

	return q.Filter(streams)
}

func (q *StreamQuery) Filter(streams []*Stream) ([]*Stream, error) {
	return q.matchCreatedPeriod(
		q.matchIdlePeriod(
			q.matchEmpty(
				q.matchConsumerLimit(
					q.matchCluster(
						q.matchServer(streams, nil),
					),
				),
			),
		),
	)
}

func (q *StreamQuery) matchCreatedPeriod(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}

	if q.CreatedPeriod == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		if (!q.Invert && time.Since(nfo.Created) >= *q.CreatedPeriod) || (q.Invert && time.Since(nfo.Created) <= *q.CreatedPeriod) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *StreamQuery) matchIdlePeriod(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}

	if q.IdlePeriod == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		lt := time.Since(state.LastTime)
		should := lt > *q.IdlePeriod
		var cerr error
		err = stream.EachConsumer(func(c *Consumer) {
			if cerr != nil {
				return
			}

			state, err := c.LatestState()
			if err != nil {
				cerr = err
				return
			}

			should = should && (state.Delivered.Last == nil || time.Since(*state.Delivered.Last) > *q.IdlePeriod)
		})
		if err != nil {
			return nil, err
		}
		if cerr != nil {
			return nil, err
		}

		if (!q.Invert && should) || (q.Invert && !should) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *StreamQuery) matchEmpty(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}

	if q.Empty == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		if (!q.Invert && state.Msgs == 0) || (q.Invert && state.Msgs > 0) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *StreamQuery) matchConsumerLimit(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}
	if q.ConsumersLimit == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		if (q.Invert && state.Consumers >= *q.ConsumersLimit) || !q.Invert && state.Consumers <= *q.ConsumersLimit {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *StreamQuery) matchCluster(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}
	if q.Cluster == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		should := false
		if nfo.Cluster != nil {
			should = q.Cluster.MatchString(nfo.Cluster.Name)
		}

		// without cluster info its included if inverted
		if (!q.Invert && should) || (q.Invert && !should) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *StreamQuery) matchServer(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}
	if q.Server == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		should := false

		if nfo.Cluster != nil {
			for _, r := range nfo.Cluster.Replicas {
				if q.Server.MatchString(r.Name) {
					should = true
					break
				}
			}

			if q.Server.MatchString(nfo.Cluster.Leader) {
				should = true
			}
		}

		// if no cluster info was present we wont include the stream
		// unless invert is set then we can
		if (!q.Invert && should) || (q.Invert && !should) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}
