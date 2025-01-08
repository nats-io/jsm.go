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

package audit

import (
	"strings"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
	"golang.org/x/exp/maps"
)

func RegisterLeafnodeChecks(collection *CheckCollection) error {
	return collection.Register(
		Check{
			Code:        "LEAF_001",
			Suite:       "leaf",
			Name:        "Whitespace in leafnode server names",
			Description: "No Leafnode contains whitespace in its name",
			Handler:     checkLeafnodeServerNamesForWhitespace,
		},
	)
}

func checkLeafnodeServerNamesForWhitespace(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)

		leafnodesWithWhitespace := map[string]struct{}{}

		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPILeafzResponse
			var serverLeafz *server.Leafz
			err := r.Load(&resp, clusterTag, serverTag, archive.TagServerLeafs())
			if err != nil {
				log.Warnf("Artifact 'LEAFZ' is missing for server %s", serverName)
				continue
			}
			serverLeafz = resp.Data

			for _, leaf := range serverLeafz.Leafs {
				// check if leafnode name contains whitespace
				if strings.ContainsAny(leaf.Name, " \n") {
					leafnodesWithWhitespace[leaf.Name] = struct{}{}
				}
			}
		}

		if len(leafnodesWithWhitespace) > 0 {
			examples.Add("Cluster %s: %v", clusterName, maps.Keys(leafnodesWithWhitespace))
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d clusters with leafnode names containing whitespace", examples.Count())
		return Fail, nil
	}

	return Pass, nil
}
