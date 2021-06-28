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
	DefaultHistory uint = 1
)

type options struct {
	history               uint
	replicas              uint
	placementCluster      string
	mirrorBucket          string
	ttl                   time.Duration
	localCache            bool
	enc                   func(string) string
	dec                   func(string) string
	log                   Logger
	timeout               time.Duration
	noShare               bool
	overrideStreamName    string
	overrideSubjectPrefix string
}

type option func(o *options) error

func newOpts(opts ...option) (*options, error) {
	o := &options{
		replicas: 1,
		history:  DefaultHistory,
		timeout:  DefaultTimeout,
		log:      &stdLogger{},
	}

	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}

// WithOutSharingClientIP disables sharing the IP of the producing client when putting values
func WithOutSharingClientIP() option {
	return func(o *options) error {
		o.noShare = false
		return nil
	}
}

// WithHistory sets the number of historic values to keep for a key
func WithHistory(h uint) option {
	return func(o *options) error {
		o.history = h
		return nil
	}
}

// WithReplicas sets the number of replicas to keep for a bucket
func WithReplicas(r uint) option {
	return func(o *options) error {
		o.replicas = r
		return nil
	}
}

// WithPlacementCluster places the bucket in a specific cluster
func WithPlacementCluster(c string) option {
	return func(o *options) error {
		o.placementCluster = c
		return nil
	}
}

// WithMirroredBucket creates a read replica that mirrors a specified bucket
func WithMirroredBucket(b string) option {
	return func(o *options) error {
		// TODO: validate
		o.mirrorBucket = b
		return nil
	}
}

// WithTTL sets the maximum time a value will be kept in the bucket
func WithTTL(ttl time.Duration) option {
	return func(o *options) error {
		o.ttl = ttl
		return nil
	}
}

// WithLocalCache creates a local in-memory cache of the entire bucket thats kept up to date in real time using a watch
func WithLocalCache() option {
	return func(o *options) error {
		o.localCache = true
		return nil
	}
}

// WithEncoderFunc sets an encoder function
func WithEncoderFunc(f func(string) string) option {
	return func(o *options) error {
		o.enc = f
		return nil
	}
}

// WithEncoder sets a value encoder, multiple encoders can be set and will be called in order, programs that just write values can use this to avoid the configuring decoders
func WithEncoder(e Encoder) option {
	return func(o *options) error {
		o.enc = e.Encode
		return nil
	}
}

// WithDecoderFunc sets an encoder function
func WithDecoderFunc(f func(string) string) option {
	return func(o *options) error {
		o.dec = f
		return nil
	}
}

// WithDecoder sets a value decoder, multiple decoders can be set and will be called in order, programs that just read values can use this to avoid the configuring encoders
func WithDecoder(d Decoder) option {
	return func(o *options) error {
		o.dec = d.Decode
		return nil
	}
}

// WithCodec sets a value encode/decoder, multiple codecs can be set and will be called in order, programs that read and write values can set this to do bi-directional encoding and decoding
func WithCodec(c Codec) option {
	return func(o *options) error {
		o.enc = c.(Encoder).Encode
		o.dec = c.(Decoder).Decode
		return nil
	}
}

// WithLogger sets a logger to use, STDOUT logging otherwise
func WithLogger(log Logger) option {
	return func(o *options) error {
		o.log = log
		return nil
	}
}

// WithTimeout sets the timeout for calls to the storage layer
func WithTimeout(t time.Duration) option {
	return func(o *options) error {
		o.timeout = t
		return nil
	}
}

// WithStreamName overrides the usual stream name that is formed as KV_<BUCKET>
func WithStreamName(n string) option {
	return func(o *options) error {
		if strings.Contains(n, ">") || strings.Contains(n, "*") || strings.Contains(n, ".") {
			return fmt.Errorf("invalid stream name")
		}

		o.overrideStreamName = n
		return nil
	}
}

// WithStreamSubjectPrefix overrides the usual stream subject changing the `kv.*.*` to `<prefix>.*.*`
func WithStreamSubjectPrefix(p string) option {
	return func(o *options) error {
		if strings.Contains(p, ">") || strings.Contains(p, "*") {
			return fmt.Errorf("invalid prefix")
		}

		p = strings.TrimSuffix(p, ".")

		o.overrideSubjectPrefix = p
		return nil
	}
}
