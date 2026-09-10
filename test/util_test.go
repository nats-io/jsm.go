// Copyright 2026 The NATS Authors
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

package test

import (
	"slices"

	"github.com/nats-io/jsm.go/api"
)

// clusterPeers is the leader and its replicas by server name
func clusterPeers(cluster *api.ClusterInfo) []string {
	peers := []string{cluster.Leader}
	for _, replica := range cluster.Replicas {
		peers = append(peers, replica.Name)
	}

	return peers
}

// settledWithoutPeer reports if the group has count current peers, none of them peer
func settledWithoutPeer(cluster *api.ClusterInfo, peer string, count int) bool {
	if cluster == nil || cluster.Leader == "" || cluster.Desired != nil {
		return false
	}

	for _, replica := range cluster.Replicas {
		if !replica.Current || replica.Offline || replica.Pending {
			return false
		}
	}

	peers := clusterPeers(cluster)
	if len(peers) != count {
		return false
	}

	return !slices.Contains(peers, peer)
}
