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

package jsm

import (
	"time"

	"github.com/nats-io/jsm.go/api"
)

// Option is a option to configure the JetStream Manager
type Option func(o *Manager)

// WithAPIValidation validates responses sent from the NATS server using a validator
func WithAPIValidation(v api.StructValidator) Option {
	return func(o *Manager) {
		o.validator = v
	}
}

// WithTrace enables logging of JSON API requests and responses
func WithTrace() Option {
	return func(o *Manager) {
		o.trace = true
	}
}

// WithTimeout sets a timeout for the requests
func WithTimeout(t time.Duration) Option {
	return func(o *Manager) {
		o.timeout = t
	}
}

// WithDefaultStream sets the default stream config when creating ad-hoc streams
func WithDefaultStream(s api.StreamConfig) Option {
	return func(o *Manager) {
		o.dfltStream = s
	}
}

// WithDefaultConsumer sets the default stream config when creating ad-hoc streams
func WithDefaultConsumer(c api.ConsumerConfig) Option {
	return func(o *Manager) {
		o.dfltConsumer = c
	}
}
