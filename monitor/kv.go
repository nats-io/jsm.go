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

package monitor

import (
	"errors"

	"github.com/nats-io/nats.go"
)

// CheckKVBucketAndKeyOptions configures the KV check
type CheckKVBucketAndKeyOptions struct {
	// Bucket is the bucket to check
	Bucket string `json:"bucket" yaml:"bucket"`
	// Key requires a key to have a non delete/purge value set
	Key string `json:"key" yaml:"key"`
	// ValuesWarning warning threshold for number of values in the bucket. Set -1 to disable
	ValuesWarning int64 `json:"values_warning" yaml:"values_warning"`
	// ValuesCritical critical threshold for number of values in the bucket. Set -1 to disable
	ValuesCritical int64 `json:"values_critical" yaml:"values_critical"`
}

func CheckKVBucketAndKey(server string, nopts []nats.Option, check *Result, opts CheckKVBucketAndKeyOptions) error {
	nc, err := nats.Connect(server, nopts...)
	if check.CriticalIfErr(err, "connection failed: %v", err) {
		return nil
	}

	js, err := nc.JetStream()
	if check.CriticalIfErr(err, "connection failed: %v", err) {
		return nil
	}

	kv, err := js.KeyValue(opts.Bucket)
	if errors.Is(err, nats.ErrBucketNotFound) {
		check.Critical("bucket %v not found", opts.Bucket)
		return nil
	} else if err != nil {
		check.Critical("could not load bucket: %v", err)
		return nil
	}

	check.Ok("bucket %s", opts.Bucket)

	status, err := kv.Status()
	if check.CriticalIfErr(err, "could not obtain bucket status: %v", err) {
		return nil
	}

	check.Pd(
		&PerfDataItem{Name: "values", Value: float64(status.Values()), Warn: float64(opts.ValuesWarning), Crit: float64(opts.ValuesCritical), Help: "How many values are stored in the bucket"},
	)

	if opts.Key != "" {
		v, err := kv.Get(opts.Key)
		if errors.Is(err, nats.ErrKeyNotFound) {
			check.Critical("key %s not found", opts.Key)
		} else if err != nil {
			check.Critical("key %s not loaded: %v", opts.Key, err)
		} else {
			switch v.Operation() {
			case nats.KeyValueDelete:
				check.Critical("key %v is deleted", opts.Key)
			case nats.KeyValuePurge:
				check.Critical("key %v is purged", opts.Key)
			case nats.KeyValuePut:
				check.Ok("key %s found", opts.Key)
			default:
				check.Critical("unknown key operation for %s: %v", opts.Key, v.Operation())
			}
		}
	}

	if opts.ValuesWarning > -1 || opts.ValuesCritical > -1 {
		if opts.ValuesCritical < opts.ValuesWarning {
			if opts.ValuesCritical > -1 && status.Values() <= uint64(opts.ValuesCritical) {
				check.Critical("%d values", status.Values())
			} else if opts.ValuesWarning > -1 && status.Values() <= uint64(opts.ValuesWarning) {
				check.Warn("%d values", status.Values())
			} else {
				check.Ok("%d values", status.Values())
			}
		} else {
			if opts.ValuesCritical > -1 && status.Values() >= uint64(opts.ValuesCritical) {
				check.Critical("%d values", status.Values())
			} else if opts.ValuesWarning > -1 && status.Values() >= uint64(opts.ValuesWarning) {
				check.Warn("%d values", status.Values())
			} else {
				check.Ok("%d values", status.Values())
			}
		}
	}

	if status.BackingStore() == "JetStream" {
		nfo := status.(*nats.KeyValueBucketStatus).StreamInfo()
		check.Pd(
			&PerfDataItem{Name: "bytes", Value: float64(nfo.State.Bytes), Unit: "B", Help: "Bytes stored in the bucket"},
			&PerfDataItem{Name: "replicas", Value: float64(nfo.Config.Replicas)},
		)
	}

	return nil
}
