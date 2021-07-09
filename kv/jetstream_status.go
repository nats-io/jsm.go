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
	"time"

	"github.com/nats-io/jsm.go/api"
)

type jsStatus struct {
	name  string
	state api.StreamState
	info  api.StreamInfo
}

func (j *jsStatus) TTL() time.Duration      { return j.info.Config.MaxAge }
func (j *jsStatus) BackingStore() string    { return j.info.Config.Name }
func (j *jsStatus) Keys() ([]string, error) { return nil, fmt.Errorf("unsupported") }
func (j *jsStatus) Bucket() string          { return j.name }
func (j *jsStatus) Values() uint64          { return j.state.Msgs }
func (j *jsStatus) History() int64          { return j.info.Config.MaxMsgsPer }
func (j *jsStatus) MaxBucketSize() int64    { return j.info.Config.MaxBytes }
func (j *jsStatus) MaxValueSize() int32     { return j.info.Config.MaxMsgSize }
func (j *jsStatus) BucketLocation() string {
	if j.info.Cluster != nil {
		return j.info.Cluster.Name
	}

	return "unknown"
}

func (j *jsStatus) Replicas() (ok int, failed int) {
	if j.info.Cluster != nil {
		for _, peer := range j.info.Cluster.Replicas {
			if peer.Current {
				ok++
			} else {
				failed++
			}
		}

	}

	return ok, failed
}
func (j *jsStatus) MirrorStatus() (lag int64, active bool, err error) {
	return 0, false, fmt.Errorf("unsupported")
}
