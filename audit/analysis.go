// Copyright 2025-2026 The NATS Authors
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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
	"golang.org/x/exp/maps"
)

// Analysis represents the result of an entire analysis
type Analysis struct {
	Type             string                `json:"type"`
	Timestamp        time.Time             `json:"time"`
	Metadata         archive.AuditMetadata `json:"metadata"`
	SkippedChecks    []string              `json:"skipped_checks"`
	SkippedSuites    []string              `json:"skipped_suites"`
	Results          []CheckResult         `json:"checks"`
	Outcomes         map[string]int        `json:"outcomes"`
	NumberedClusters []SharedClusterName   `json:"numbered_clusters,omitempty"`
}

// SharedClusterName is a cluster name and domain used by clusters that are not routed to each other
type SharedClusterName struct {
	Domain   string            `json:"domain,omitempty"`
	Name     string            `json:"name"`
	Clusters []NumberedCluster `json:"clusters"`
}

// NumberedCluster is one of the clusters sharing a name, labeled with the number the archive assigned to it
type NumberedCluster struct {
	Label   string   `json:"label"`
	Servers []string `json:"servers"`
}

func (s SharedClusterName) Description() string {
	labels := make([]string, len(s.Clusters))
	for i, cluster := range s.Clusters {
		labels[i] = cluster.Label
	}
	name := s.Name
	if s.Domain != "" {
		name = fmt.Sprintf("%s in domain %s", s.Name, s.Domain)
	}
	return fmt.Sprintf("Cluster name %s is used by %d clusters that are not routed to each other. This report numbers them %s. The numbers are not cluster names.",
		name, len(s.Clusters), strings.Join(labels, ", "))
}

func sharedClusterNames(ar *archive.Reader) []SharedClusterName {
	var shared []SharedClusterName
	for _, cluster := range ar.Clusters() {
		if cluster.Index == "" {
			continue
		}

		if len(shared) == 0 || shared[len(shared)-1].Name != cluster.Name || shared[len(shared)-1].Domain != cluster.Domain {
			shared = append(shared, SharedClusterName{Domain: cluster.Domain, Name: cluster.Name})
		}

		numbered := NumberedCluster{Label: cluster.NumberedName()}
		for _, serverName := range ar.ClusterServerNames(cluster) {
			var id string
			archive.ForEachServerArtifact(ar, cluster, serverName, []*archive.Tag{archive.TagServerVars()}, func(vz *server.ServerAPIVarzResponse) error {
				if vz.Server != nil {
					id = vz.Server.ID
				}
				return nil
			})
			numbered.Servers = append(numbered.Servers, archive.ServerLabel(serverName, id))
		}

		last := &shared[len(shared)-1]
		last.Clusters = append(last.Clusters, numbered)
	}
	return shared
}

const MarkdownFormatTemplate = `# NATS Audit Report produced {{ .Timestamp | ft}}

## Connection Details

Report generated using archive from **{{.Metadata.ConnectURL}}** by **{{.Metadata.UserName}}** created **{{.Metadata.Timestamp | ft}}**

{{- if .NumberedClusters }}

## Clusters Sharing a Name
{{   range .NumberedClusters }}
{{ .Description }}
{{     range .Clusters }}
* **{{ .Label }}**: {{ join .Servers ", " }}
{{-    end }}
{{   end }}
{{- end }}

## Report Summary

|Status|Count|
|------|-----|
|FAIL|{{index .Outcomes "FAIL"}}|
|WARN|{{index .Outcomes "WARN"}}|
|PASS|{{index .Outcomes "PASS"}}|
|SKIP|{{index .Outcomes "SKIP"}}|

## Results
{{- $suites := . | bySuite -}}
{{ range (. | suiteNames ) }}
### Check Suite: {{ . }}
{{- $results := index $suites . -}}
{{   range $results }}
#### {{ .Check.Name }}

Outcome: **{{ .OutcomeString }}**
{{     if .Examples.Examples }}
|#|Example|
|-|-------|
{{       range $index, $example := (.Examples.Examples | limitStrings ) -}}
|{{ add1 $index }}|{{ $example | escapeMarkdown }}|
{{        end -}}
{{-     end -}}
{{-   end -}}
{{- end -}}
`

const analysisType = "io.nats.audit.v1.analysis"

// LoadAnalysis loads an analysis report from a file
func LoadAnalysis(path string) (*Analysis, error) {
	ab, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := Analysis{}
	if err = json.Unmarshal(ab, &result); err != nil {
		return nil, err
	}

	if result.Type != analysisType {
		return nil, fmt.Errorf("unexpected analysis type %q", result.Type)
	}

	return &result, nil
}

// ToJSON renders the report in JSON format
func (a *Analysis) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a, "", "   ")
}

// ToMarkdown produces a markdown report with examples limited to limitExamples (0 for unlimited)
func (a *Analysis) ToMarkdown(templ string, limitExamples uint) ([]byte, error) {
	if templ == "" {
		return nil, fmt.Errorf("template must not be empty")
	}

	t, err := template.New("report.md").Funcs(template.FuncMap{
		"ft":             func(t time.Time) string { return t.Format(time.RFC822Z) },
		"bySuite":        resultsBySuite,
		"suiteNames":     suiteNames,
		"add1":           func(i int) int { return i + 1 },
		"join":           strings.Join,
		"escapeMarkdown": func(s string) string { return strings.ReplaceAll(s, "|", `\|`) },
		"limitStrings": func(a []string) []string {
			if limitExamples == 0 || uint(len(a)) < limitExamples {
				return a
			}
			return a[0:limitExamples]
		},
	}).Parse(templ)
	if err != nil {
		return nil, err
	}

	out := &bytes.Buffer{}
	if err = t.Execute(out, a); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func resultsBySuite(a *Analysis) map[string][]CheckResult {
	suites := map[string][]CheckResult{}
	for _, result := range a.Results {
		suites[result.Check.Suite] = append(suites[result.Check.Suite], result)
	}

	for suite := range suites {
		sort.Slice(suites[suite], func(i, j int) bool {
			return suites[suite][i].Check.Code < suites[suite][j].Check.Code
		})
	}

	return suites
}

func suiteNames(a *Analysis) []string {
	suites := map[string]struct{}{}
	for _, result := range a.Results {
		suites[result.Check.Suite] = struct{}{}
	}

	names := maps.Keys(suites)
	sort.Strings(names)

	return names
}
