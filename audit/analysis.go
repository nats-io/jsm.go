// Copyright 2025 The NATS Authors
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
	"os"
	"slices"
	"sort"
	"text/template"
	"time"

	"github.com/nats-io/jsm.go/audit/archive"
	"golang.org/x/exp/maps"
)

// Analysis represents the result of an entire analysis
type Analysis struct {
	Type          string                `json:"type"`
	Timestamp     time.Time             `json:"time"`
	Metadata      archive.AuditMetadata `json:"metadata"`
	SkippedChecks []string              `json:"skipped_checks"`
	SkippedSuites []string              `json:"skipped_suites"`
	Results       []CheckResult         `json:"checks"`
	Outcomes      map[string]int        `json:"outcomes"`
}

var MarkdownFormatTemplate = `# NATS Audit Report produced {{ .Timestamp | ft}}

## Connection Details

Report generated using archive from **{{.Metadata.ConnectURL}}** by **{{.Metadata.UserName}}** created **{{.Metadata.Timestamp | ft}}**

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
|Count|Example|
|-----|-------|
{{       range $index, $example := (.Examples.Examples | limitStrings ) -}}
|{{ $index }}|{{ $example }}|
{{        end -}}
{{-     end -}}
{{-   end -}}
{{- end -}}
`

// LoadAnalysis loads an analysis report from a file
func LoadAnalysis(path string) (*Analysis, error) {
	ab, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	analyzes := Analysis{}
	err = json.Unmarshal(ab, &analyzes)
	if err != nil {
		return nil, err
	}

	return &analyzes, nil
}

// ToJSON renders the report in JSON format
func (a *Analysis) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a, "", "   ")
}

// ToMarkdown produce a markdown report with examples limited to limitExamples (0 for unlimited)
func (a *Analysis) ToMarkdown(templ string, limitExamples uint) ([]byte, error) {
	t, err := template.New("report.md").Funcs(template.FuncMap{
		"ft":         func(t time.Time) string { return t.Format(time.RFC822Z) },
		"bySuite":    resultsBySuite,
		"suiteNames": suiteNames,
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
	err = t.Execute(out, a)
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func resultsBySuite(a *Analysis) map[string][]CheckResult {
	suites := map[string][]CheckResult{}
	for _, result := range a.Results {
		suites[result.Check.Suite] = append(suites[result.Check.Suite], result)
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

	return slices.Compact(names)
}
