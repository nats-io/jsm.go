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

	"github.com/nats-io/jsm.go/api"
)

type jsStatus struct {
	state  api.StreamState
	config api.StreamConfig
}

func (j *jsStatus) Values() uint64 { return j.state.Msgs }
func (j *jsStatus) History() int64 { return j.config.MaxMsgsPer }
func (j *jsStatus) Cluster() string {
	if j.config.Placement == nil {
		return ""
	}
	return j.config.Placement.Cluster
}
func (j *jsStatus) Replicas() int           { return j.config.Replicas }
func (j *jsStatus) Keys() ([]string, error) { return nil, fmt.Errorf("unsupported") }
func (j *jsStatus) MirrorStatus() (lag int64, active bool, err error) {
	return 0, false, fmt.Errorf("unsupported")
}
