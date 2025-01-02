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
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"golang.org/x/exp/maps"
)

// CheckFunc implements a check over gathered audit
type CheckFunc func(check *Check, reader *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error)

// Check is the basic unit of analysis that is run against a data archive
type Check struct {
	Code          string                         `json:"code"`
	Suite         string                         `json:"suite"`
	Name          string                         `json:"name"`
	Description   string                         `json:"description"`
	Configuration map[string]*CheckConfiguration `json:"configuration"`
	Handler       CheckFunc                      `json:"-"`
}

// CheckCollection is a collection holding registered checks
type CheckCollection struct {
	registered    map[string]*Check
	configuration map[string]*CheckConfiguration
	suites        map[string][]*Check
	skipCheck     []string
	skipSuite     []string
	mu            sync.Mutex
}

// SkipChecks marks certain tests to be skipped while running the collection
func (c *CheckCollection) SkipChecks(checks ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, check := range checks {
		if !slices.Contains(c.skipCheck, check) {
			c.skipCheck = append(c.skipCheck, check)
		}
	}
}

// SkipSuites marks certain test suites to be skipped while running the collection
func (c *CheckCollection) SkipSuites(suites ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, suite := range suites {
		if !slices.Contains(c.skipSuite, suite) {
			c.skipSuite = append(c.skipSuite, suite)
		}
	}
}

// Register adds a check to the collection
func (c *CheckCollection) Register(checks ...Check) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.registered == nil {
		c.registered = make(map[string]*Check)
	}
	if c.suites == nil {
		c.suites = make(map[string][]*Check)
	}
	if c.configuration == nil {
		c.configuration = make(map[string]*CheckConfiguration)
	}

	for _, check := range checks {
		if check.Code == "" {
			return fmt.Errorf("check code is required")
		}
		if check.Suite == "" {
			return fmt.Errorf("check suite is required")
		}
		if check.Name == "" {
			return fmt.Errorf("check name is required")
		}
		if check.Description == "" {
			return fmt.Errorf("check description is required")
		}
		if check.Handler == nil {
			return fmt.Errorf("check implementation is required")
		}
		if check.Configuration == nil {
			check.Configuration = make(map[string]*CheckConfiguration)
		}

		if _, ok := c.registered[check.Name]; ok {
			return fmt.Errorf("check %q already registered", check.Name)
		}

		for _, cfg := range check.Configuration {
			if cfg.Key == "" {
				return fmt.Errorf("configuration key is required")
			}
			if cfg.Description == "" {
				return fmt.Errorf("configuration description is required")
			}

			cfg.Check = check.Code
			c.configuration[configItemKey(check.Code, cfg.Key)] = cfg
		}

		c.registered[check.Name] = &check
		if _, ok := c.suites[check.Suite]; !ok {
			c.suites[check.Suite] = []*Check{}
		}
		c.suites[check.Suite] = append(c.suites[check.Suite], &check)
	}

	return nil
}

// MewCollection creates a new collection with no checks loaded
func MewCollection() *CheckCollection {
	return &CheckCollection{}
}

// NewDefaultCheckCollection creates a new collection and loads the standard set of checks
func NewDefaultCheckCollection() (*CheckCollection, error) {
	c := &CheckCollection{}

	for _, f := range []func(*CheckCollection) error{
		RegisterAccountChecks,
		RegisterClusterChecks,
		RegisterLeafnodeChecks,
		RegisterMetaChecks,
		RegisterServerChecks,
		RegisterJetStreamChecks,
	} {
		err := f(c)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

// MustRegister calls Register and panics on error
func (c *CheckCollection) MustRegister(checks ...Check) {
	err := c.Register(checks...)
	if err != nil {
		panic(err)
	}
}

func configItemKey(code string, key string) string {
	return fmt.Sprintf("%s_%s", strings.ToLower(code), key)
}

// Outcome of running a check against the data gathered into an archive
type Outcome int

const (
	// Pass is for no issues detected
	Pass Outcome = iota
	// PassWithIssues is for non-critical problems
	PassWithIssues Outcome = iota
	// Fail indicates a bad state is detected
	Fail Outcome = iota
	// Skipped is for checks that failed to run (no data, runtime error, ...)
	Skipped Outcome = iota
)

// Outcomes is the list of possible outcomes values
var Outcomes = [...]Outcome{
	Pass,
	PassWithIssues,
	Fail,
	Skipped,
}

// String converts an outcome into a 4-letter string value
func (o Outcome) String() string {
	switch o {
	case Fail:
		return "FAIL"
	case Pass:
		return "PASS"
	case PassWithIssues:
		return "WARN"
	case Skipped:
		return "SKIP"
	default:
		panic(fmt.Sprintf("Uknown outcome code: %d", o))
	}
}

// ConfigurationItems loads a list of config items sorted by check
//
// Use in fisk applications like:
//
//	 cfg := audit.ConfigurationItems()
//	 for _, v := range cfg {
//		v.SetVal(analyze.Flag(fmt.Sprintf("%s_%s", strings.ToLower(v.Check), v.Key), v.Description).Default(fmt.Sprintf("%.2f", v.Default)))
//	 }
func (c *CheckCollection) ConfigurationItems() []*CheckConfiguration {
	var res []*CheckConfiguration

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, check := range c.configuration {
		res = append(res, check)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Key < res[j].Key
	})

	return res
}

// runCheck is a wrapper to run a check, handling setup and errors
func runCheck(check *Check, ar *archive.Reader, limit uint, log api.Logger) (Outcome, *ExamplesCollection) {
	examples := newExamplesCollection(limit)
	outcome, err := check.Handler(check, ar, examples, log)
	if err != nil {
		examples.Error = err.Error()
		return Skipped, examples
	}
	return outcome, examples
}

// CheckResult is a outcome of a single check
type CheckResult struct {
	Check         Check              `json:"check"`
	Outcome       Outcome            `json:"outcome"`
	OutcomeString string             `json:"outcome_string"`
	Examples      ExamplesCollection `json:"examples"`
}

// Analysis represents the result of an entire analysis
type Analysis struct {
	Type          string                `json:"type"`
	Time          time.Time             `json:"time"`
	Metadata      archive.AuditMetadata `json:"metadata"`
	SkippedChecks []string              `json:"skipped_checks"`
	SkippedSuites []string              `json:"skipped_suites"`
	Results       []CheckResult         `json:"checks"`
	Outcomes      map[string]int        `json:"outcomes"`
}

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

func (c *CheckCollection) EachCheck(cb func(c *Check)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	suites := maps.Keys(c.suites)
	sort.Strings(suites)

	for _, suite := range suites {
		sort.Slice(c.suites[suite], func(i, j int) bool {
			return c.suites[suite][i].Name < c.suites[suite][j].Name
		})

		for _, check := range c.suites[suite] {
			cb(check)
		}
	}
}

func (c *CheckCollection) Run(ar *archive.Reader, limit uint, log api.Logger) *Analysis {
	result := &Analysis{
		Type:          "io.nats.audit.v1.analysis",
		Time:          time.Now().UTC(),
		SkippedChecks: c.skipCheck,
		SkippedSuites: c.skipSuite,
		Results:       []CheckResult{},
		Outcomes:      make(map[string]int),
	}

	if result.SkippedChecks == nil {
		result.SkippedChecks = []string{}
	}
	sort.Strings(result.SkippedChecks)
	if result.SkippedSuites == nil {
		result.SkippedSuites = []string{}
	}
	sort.Strings(result.SkippedSuites)

	ar.Load(&result.Metadata, archive.TagSpecial("audit_gather_metadata"))

	for _, outcome := range Outcomes {
		result.Outcomes[outcome.String()] = 0
	}

	c.EachCheck(func(check *Check) {
		should := !slices.ContainsFunc(c.skipCheck, func(s string) bool {
			return strings.EqualFold(check.Code, s)
		})

		should = should && !slices.ContainsFunc(c.skipSuite, func(s string) bool {
			return strings.EqualFold(check.Suite, s)
		})

		var res CheckResult
		if should {
			outcome, examples := runCheck(check, ar, limit, log)
			res = CheckResult{
				Check:   *check,
				Outcome: outcome,
			}

			if examples != nil && len(examples.Examples) > 0 {
				res.Examples = *examples
			}
		} else {
			res = CheckResult{
				Check:   *check,
				Outcome: Skipped,
			}
		}

		res.OutcomeString = res.Outcome.String()

		result.Results = append(result.Results, res)
		result.Outcomes[res.Outcome.String()]++
	})

	return result
}
