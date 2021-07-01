// Copyright 2021 The NATS Authors
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

package kv

import (
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultTimeout is the default timeout used when waiting for the backend, override using WithTimeout()
	DefaultTimeout = 2 * time.Second

	// DefaultHistory is how many historical values are kept per key
	DefaultHistory uint64 = 1

	// DefaultMaxBucketSize maximum size for the entire bucket, -1 for unlimited
	DefaultMaxBucketSize int64 = -1

	// DefaultMaxValueSize maximum size for individual values
	DefaultMaxValueSize int32 = -1
)

type options struct {
	history               uint64
	replicas              uint
	maxBucketSize         int64
	maxValueSize          int32
	placementCluster      string
	mirrorBucket          string
	ttl                   time.Duration
	localCache            bool
	enc                   Encoder
	dec                   Decoder
	log                   Logger
	timeout               time.Duration
	overrideStreamName    string
	overrideSubjectPrefix string
}

// Option configures the KV client
type Option func(o *options) error

// PutOption is a option passed to put, reserved for future work like put only if last value had sequence x
type PutOption func(o *putOptions)

type putOptions struct {
	jsPreviousSeq uint64
}

func newOpts(opts ...Option) (*options, error) {
	o := &options{
		replicas:      1,
		history:       DefaultHistory,
		timeout:       DefaultTimeout,
		maxBucketSize: DefaultMaxBucketSize,
		maxValueSize:  DefaultMaxValueSize,
		log:           &stdLogger{},
	}

	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}

func newPutOpts(opts ...PutOption) (*putOptions, error) {
	o := &putOptions{}

	for _, opt := range opts {
		opt(o)
	}

	return o, nil
}

// WithHistory sets the number of historic values to keep for a key
func WithHistory(h uint64) Option {
	return func(o *options) error {
		o.history = h
		return nil
	}
}

// WithMaxValueSize is the biggest size value that can be placed in the bucket including some header overhead
func WithMaxValueSize(s int32) Option {
	return func(o *options) error {
		if s < -1 {
			return fmt.Errorf("minimum value for WithMaxValueSize is -1")
		}

		o.maxValueSize = s
		return nil
	}
}

// WithMaxBucketSize limits the entire bucket to a specific size
func WithMaxBucketSize(s int64) Option {
	return func(o *options) error {
		if s < -1 {
			return fmt.Errorf("minimum value for WithMaxBucketSize is -1")
		}

		o.maxBucketSize = s
		return nil
	}
}

// WithReplicas sets the number of replicas to keep for a bucket
func WithReplicas(r uint) Option {
	return func(o *options) error {
		o.replicas = r
		return nil
	}
}

// WithPlacementCluster places the bucket in a specific cluster
func WithPlacementCluster(c string) Option {
	return func(o *options) error {
		o.placementCluster = c
		return nil
	}
}

// WithMirroredBucket creates a read replica that mirrors a specified bucket
func WithMirroredBucket(b string) Option {
	return func(o *options) error {
		// TODO: validate
		o.mirrorBucket = b
		return nil
	}
}

// WithTTL sets the maximum time a value will be kept in the bucket
func WithTTL(ttl time.Duration) Option {
	return func(o *options) error {
		o.ttl = ttl
		return nil
	}
}

// WithLocalCache creates a local in-memory cache of the entire bucket that's kept up to date in real time using a watch
func WithLocalCache() Option {
	return func(o *options) error {
		o.localCache = true
		return nil
	}
}

// WithEncoder sets a value encoder, multiple encoders can be set and will be called in order, programs that just write values can use this to avoid the configuring decoders
func WithEncoder(e Encoder) Option {
	return func(o *options) error {
		o.enc = e
		return nil
	}
}

// WithDecoder sets a value decoder, multiple decoders can be set and will be called in order, programs that just read values can use this to avoid the configuring encoders
func WithDecoder(d Decoder) Option {
	return func(o *options) error {
		o.dec = d
		return nil
	}
}

// WithCodec sets a value encode/decoder, multiple codecs can be set and will be called in order, programs that read and write values can set this to do bi-directional encoding and decoding
func WithCodec(c Codec) Option {
	return func(o *options) error {
		o.enc = c.(Encoder)
		o.dec = c.(Decoder)
		return nil
	}
}

// WithLogger sets a logger to use, STDOUT logging otherwise
func WithLogger(log Logger) Option {
	return func(o *options) error {
		o.log = log
		return nil
	}
}

// WithTimeout sets the timeout for calls to the storage layer
func WithTimeout(t time.Duration) Option {
	return func(o *options) error {
		o.timeout = t
		return nil
	}
}

// WithStreamName overrides the usual stream name that is formed as KV_<BUCKET>
func WithStreamName(n string) Option {
	return func(o *options) error {
		if strings.Contains(n, ">") || strings.Contains(n, "*") || strings.Contains(n, ".") {
			return fmt.Errorf("invalid stream name")
		}

		o.overrideStreamName = n
		return nil
	}
}

// WithStreamSubjectPrefix overrides the usual stream subject changing the `kv.*.*` to `<prefix>.*.*`
func WithStreamSubjectPrefix(p string) Option {
	return func(o *options) error {
		if strings.Contains(p, ">") || strings.Contains(p, "*") {
			return fmt.Errorf("invalid prefix")
		}

		p = strings.TrimSuffix(p, ".")

		o.overrideSubjectPrefix = p
		return nil
	}
}

// OnlyIfLastValueSequence the put will only succeed if the last set value for the key had this sequence
func OnlyIfLastValueSequence(seq uint64) PutOption {
	return func(o *putOptions) {
		o.jsPreviousSeq = seq
	}
}
