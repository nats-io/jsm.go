// Copyright 2020 The NATS Authors
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

package election

import (
	"fmt"
	"time"

	"github.com/nats-io/jsm.go"
)

const (
	// DefaultCampaignInterval how frequently campaigners will try to make the consumer
	DefaultCampaignInterval = 500 * time.Millisecond

	// DefaultHeartBeatInterval how frequently the consumer will inform the leader that it's still alive
	DefaultHeartBeatInterval = 500 * time.Millisecond

	// DefaultMissedHBThreshold how many heartbeats we can miss before we decide we lost leadership
	DefaultMissedHBThreshold = 3
)

type options struct {
	name              string
	wonCb             func()
	lostCb            func()
	campaignInterval  time.Duration
	hbInterval        time.Duration
	missedHBThreshold int
	stream            *jsm.Stream
	bo                Backoff
}

// WithBackoff will use the provided Backoff timer source to decrease campaign intervals over time
func WithBackoff(bo Backoff) Option {
	return func(o *options) error {
		o.bo = bo

		return nil
	}
}

// WithStream sets a specific stream as campaign target, must have max_consumers=1
func WithStream(stream *jsm.Stream) Option {
	return func(o *options) error {
		if stream.MaxConsumers() != 1 {
			return fmt.Errorf("stream should have 1 maximum consumer")
		}

		o.stream = stream

		return nil
	}
}

// WithHeartBeatInterval sets the interval JetStream will notify the leader that it's still active
func WithHeartBeatInterval(i time.Duration) Option {
	return func(o *options) error {
		if i <= DefaultHeartBeatInterval {
			return fmt.Errorf("heartbeat interval too small")
		}

		o.hbInterval = i

		return nil
	}
}

// WithCampaignInterval sets the interval at which campaigners will attempt to become leader
func WithCampaignInterval(i time.Duration) Option {
	return func(o *options) error {
		if i <= DefaultCampaignInterval {
			return fmt.Errorf("campaign interval too small")
		}

		o.campaignInterval = i

		return nil
	}

}

// WithMissedHeartbeatThreshold configures how many heartbeats we can miss from JetStream
func WithMissedHeartbeatThreshold(t int) Option {
	return func(o *options) error {
		if t < DefaultMissedHBThreshold {
			return fmt.Errorf("missed heartbeat threshold too small")
		}

		o.missedHBThreshold = t

		return nil
	}
}

// Option configures the leader election system
type Option func(*options) error

func newOptions(opts ...Option) (*options, error) {
	o := &options{
		campaignInterval:  DefaultCampaignInterval,
		hbInterval:        DefaultHeartBeatInterval,
		missedHBThreshold: DefaultMissedHBThreshold,
	}

	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}
