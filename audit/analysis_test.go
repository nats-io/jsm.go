package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/audit/archive"
)

func sampleAnalysis() *Analysis {
	return &Analysis{
		Type:          "io.nats.audit.v1.analysis",
		Timestamp:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		SkippedChecks: []string{},
		SkippedSuites: []string{},
		Metadata: archive.AuditMetadata{
			ConnectURL: "nats://localhost:4222",
			UserName:   "admin",
		},
		Results: []CheckResult{
			{
				Check:         Check{Code: "CLUSTER_001", Suite: "cluster", Name: "Cluster Memory"},
				Outcome:       Pass,
				OutcomeString: "PASS",
			},
			{
				Check:         Check{Code: "ACCOUNTS_001", Suite: "accounts", Name: "Account Limits"},
				Outcome:       PassWithIssues,
				OutcomeString: "WARN",
				Examples: ExamplesCollection{
					Examples: []string{"account SYS using 95.0% of client connections limit (950/1000)"},
				},
			},
		},
		Outcomes: map[string]int{"PASS": 1, "WARN": 1, "FAIL": 0, "SKIP": 0},
	}
}

func TestLoadAnalysis(t *testing.T) {
	t.Run("loads a valid analysis file", func(t *testing.T) {
		tmp := t.TempDir()
		a := sampleAnalysis()

		b, err := a.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON failed: %v", err)
		}

		path := filepath.Join(tmp, "analysis.json")
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		loaded, err := LoadAnalysis(path)
		if err != nil {
			t.Fatalf("LoadAnalysis failed: %v", err)
		}
		if loaded.Type != a.Type {
			t.Errorf("Type: expected %q, got %q", a.Type, loaded.Type)
		}
		if len(loaded.Results) != len(a.Results) {
			t.Errorf("Results: expected %d, got %d", len(a.Results), len(loaded.Results))
		}
	})

	t.Run("errors on missing file", func(t *testing.T) {
		_, err := LoadAnalysis("/nonexistent/path/analysis.json")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("errors on invalid JSON", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "bad.json")
		if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		_, err := LoadAnalysis(path)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("errors on wrong type field", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "wrong.json")
		if err := os.WriteFile(path, []byte(`{"type":"wrong.type"}`), 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		_, err := LoadAnalysis(path)
		if err == nil {
			t.Error("expected error for wrong type")
		}
	})

	t.Run("errors when type field is absent", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "notype.json")
		if err := os.WriteFile(path, []byte(`{"checks":[]}`), 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		_, err := LoadAnalysis(path)
		if err == nil {
			t.Error("expected error for absent type field")
		}
	})
}

func TestAnalysisToJSON(t *testing.T) {
	t.Run("produces non-empty JSON", func(t *testing.T) {
		b, err := sampleAnalysis().ToJSON()
		if err != nil {
			t.Fatalf("ToJSON failed: %v", err)
		}
		if len(b) == 0 {
			t.Error("expected non-empty JSON output")
		}
	})

	t.Run("round-trips through LoadAnalysis", func(t *testing.T) {
		tmp := t.TempDir()
		a := sampleAnalysis()

		b, err := a.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON failed: %v", err)
		}

		path := filepath.Join(tmp, "round.json")
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		loaded, err := LoadAnalysis(path)
		if err != nil {
			t.Fatalf("LoadAnalysis failed: %v", err)
		}
		if loaded.Type != a.Type {
			t.Errorf("Type: expected %q, got %q", a.Type, loaded.Type)
		}
		if loaded.Outcomes["PASS"] != a.Outcomes["PASS"] {
			t.Errorf("Outcomes[PASS]: expected %d, got %d", a.Outcomes["PASS"], loaded.Outcomes["PASS"])
		}
		if loaded.Results[0].Check.Code != a.Results[0].Check.Code {
			t.Errorf("Results[0].Check.Code: expected %q, got %q", a.Results[0].Check.Code, loaded.Results[0].Check.Code)
		}
	})
}

func TestAnalysisToMarkdown(t *testing.T) {
	t.Run("MarkdownFormatTemplate is valid and produces output", func(t *testing.T) {
		b, err := sampleAnalysis().ToMarkdown(MarkdownFormatTemplate, 0)
		if err != nil {
			t.Fatalf("ToMarkdown with MarkdownFormatTemplate failed: %v", err)
		}
		if len(b) == 0 {
			t.Error("expected non-empty markdown output")
		}
	})

	t.Run("output contains suite and check names", func(t *testing.T) {
		b, err := sampleAnalysis().ToMarkdown(MarkdownFormatTemplate, 0)
		if err != nil {
			t.Fatalf("ToMarkdown failed: %v", err)
		}
		out := string(b)
		for _, want := range []string{"accounts", "cluster", "Account Limits", "Cluster Memory"} {
			if !strings.Contains(out, want) {
				t.Errorf("expected output to contain %q", want)
			}
		}
	})

	t.Run("limitExamples=0 includes all examples", func(t *testing.T) {
		a := sampleAnalysis()
		a.Results[1].Examples.Examples = []string{"ex1", "ex2", "ex3", "ex4", "ex5"}

		b, err := a.ToMarkdown(MarkdownFormatTemplate, 0)
		if err != nil {
			t.Fatalf("ToMarkdown failed: %v", err)
		}
		out := string(b)
		for _, ex := range []string{"ex1", "ex2", "ex3", "ex4", "ex5"} {
			if !strings.Contains(out, ex) {
				t.Errorf("expected output to contain %q", ex)
			}
		}
	})

	t.Run("limitExamples caps examples in output", func(t *testing.T) {
		a := sampleAnalysis()
		a.Results[1].Examples.Examples = []string{"ex1", "ex2", "ex3", "ex4", "ex5"}

		b, err := a.ToMarkdown(MarkdownFormatTemplate, 2)
		if err != nil {
			t.Fatalf("ToMarkdown failed: %v", err)
		}
		out := string(b)
		if !strings.Contains(out, "ex1") || !strings.Contains(out, "ex2") {
			t.Error("expected first two examples in output")
		}
		if strings.Contains(out, "ex3") {
			t.Error("expected ex3 to be omitted when limit=2")
		}
	})

	t.Run("pipe characters in examples are escaped", func(t *testing.T) {
		a := sampleAnalysis()
		a.Results[1].Examples.Examples = []string{"account foo|bar on server s1"}

		b, err := a.ToMarkdown(MarkdownFormatTemplate, 0)
		if err != nil {
			t.Fatalf("ToMarkdown failed: %v", err)
		}
		if !strings.Contains(string(b), `foo\|bar`) {
			t.Errorf("expected pipe to be escaped in markdown table, got:\n%s", b)
		}
	})

	t.Run("example index is 1-based", func(t *testing.T) {
		a := sampleAnalysis()
		a.Results[1].Examples.Examples = []string{"first example"}

		b, err := a.ToMarkdown(MarkdownFormatTemplate, 0)
		if err != nil {
			t.Fatalf("ToMarkdown failed: %v", err)
		}
		out := string(b)
		if !strings.Contains(out, "|1|") {
			t.Errorf("expected 1-based index in table, got:\n%s", out)
		}
		if strings.Contains(out, "|0|first example|") {
			t.Errorf("unexpected 0-based index in examples table, got:\n%s", out)
		}
	})

	t.Run("errors on empty template", func(t *testing.T) {
		_, err := sampleAnalysis().ToMarkdown("", 0)
		if err == nil {
			t.Error("expected error for empty template")
		}
	})

	t.Run("errors on invalid template syntax", func(t *testing.T) {
		_, err := sampleAnalysis().ToMarkdown("{{invalid", 0)
		if err == nil {
			t.Error("expected error for invalid template syntax")
		}
	})
}

func TestResultsBySuite(t *testing.T) {
	t.Run("groups results by suite", func(t *testing.T) {
		suites := resultsBySuite(sampleAnalysis())
		if len(suites["cluster"]) != 1 {
			t.Errorf("expected 1 cluster result, got %d", len(suites["cluster"]))
		}
		if len(suites["accounts"]) != 1 {
			t.Errorf("expected 1 accounts result, got %d", len(suites["accounts"]))
		}
	})

	t.Run("results within each suite are sorted by check code", func(t *testing.T) {
		a := &Analysis{
			Results: []CheckResult{
				{Check: Check{Code: "CLUSTER_003", Suite: "cluster", Name: "Three"}},
				{Check: Check{Code: "CLUSTER_001", Suite: "cluster", Name: "One"}},
				{Check: Check{Code: "CLUSTER_002", Suite: "cluster", Name: "Two"}},
			},
		}
		results := resultsBySuite(a)["cluster"]
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
		codes := []string{results[0].Check.Code, results[1].Check.Code, results[2].Check.Code}
		expected := []string{"CLUSTER_001", "CLUSTER_002", "CLUSTER_003"}
		for i := range expected {
			if codes[i] != expected[i] {
				t.Errorf("position %d: expected %q, got %q", i, expected[i], codes[i])
			}
		}
	})

	t.Run("empty results gives empty map", func(t *testing.T) {
		if suites := resultsBySuite(&Analysis{}); len(suites) != 0 {
			t.Errorf("expected empty map, got %v", suites)
		}
	})
}

func TestSuiteNames(t *testing.T) {
	t.Run("returns sorted unique suite names", func(t *testing.T) {
		a := &Analysis{
			Results: []CheckResult{
				{Check: Check{Suite: "cluster"}},
				{Check: Check{Suite: "accounts"}},
				{Check: Check{Suite: "cluster"}}, // duplicate — should be deduplicated
				{Check: Check{Suite: "jetstream"}},
			},
		}
		names := suiteNames(a)
		expected := []string{"accounts", "cluster", "jetstream"}
		if len(names) != len(expected) {
			t.Fatalf("expected %v, got %v", expected, names)
		}
		for i, name := range names {
			if name != expected[i] {
				t.Errorf("position %d: expected %q, got %q", i, expected[i], name)
			}
		}
	})

	t.Run("empty results returns empty slice", func(t *testing.T) {
		if names := suiteNames(&Analysis{}); len(names) != 0 {
			t.Errorf("expected empty slice, got %v", names)
		}
	})
}
