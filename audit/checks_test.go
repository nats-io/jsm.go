package audit

import (
	"path/filepath"
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
)

// emptyArchive creates a minimal archive with no data artifacts and returns an open reader.
func emptyArchive(t *testing.T) *archive.Reader {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.zip")

	w, err := archive.NewWriter(path)
	if err != nil {
		t.Fatalf("failed to create archive writer: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close archive writer: %v", err)
	}

	r, err := archive.NewReader(path)
	if err != nil {
		t.Fatalf("failed to open archive reader: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

// stubCheck builds a minimal valid Check whose handler always returns the given outcome.
func stubCheck(code, suite, name string, outcome Outcome) Check {
	return Check{
		Code:        code,
		Suite:       suite,
		Name:        name,
		Description: "stub check for testing",
		Handler: func(_ *Check, _ *archive.Reader, _ *ExamplesCollection, _ api.Logger) (Outcome, error) {
			return outcome, nil
		},
	}
}

// --- Register ---

func TestRegister_Valid(t *testing.T) {
	cc := &CheckCollection{}
	if err := cc.Register(stubCheck("TST_001", "test", "Test One", Pass)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegister_DeduplicatesOnCode(t *testing.T) {
	t.Run("same code different name is rejected", func(t *testing.T) {
		cc := &CheckCollection{}
		if err := cc.Register(stubCheck("TST_001", "test", "First Name", Pass)); err != nil {
			t.Fatalf("first register failed: %v", err)
		}
		second := stubCheck("TST_001", "test", "Second Name", Pass)
		if err := cc.Register(second); err == nil {
			t.Error("expected error registering duplicate code, got nil")
		}
	})

	t.Run("same name different code both register", func(t *testing.T) {
		cc := &CheckCollection{}
		if err := cc.Register(stubCheck("TST_001", "test", "Shared Name", Pass)); err != nil {
			t.Fatalf("first register failed: %v", err)
		}
		second := stubCheck("TST_002", "test", "Shared Name", Pass)
		// Different codes should both be accepted even if names collide
		if err := cc.Register(second); err != nil {
			t.Fatalf("second register with different code failed: %v", err)
		}
	})
}

func TestRegister_RequiredFields(t *testing.T) {
	base := stubCheck("TST_001", "test", "Test One", Pass)

	cases := []struct {
		name   string
		mutate func(*Check)
	}{
		{"missing code", func(c *Check) { c.Code = "" }},
		{"missing suite", func(c *Check) { c.Suite = "" }},
		{"missing name", func(c *Check) { c.Name = "" }},
		{"missing description", func(c *Check) { c.Description = "" }},
		{"missing handler", func(c *Check) { c.Handler = nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := base
			tc.mutate(&check)
			cc := &CheckCollection{}
			if err := cc.Register(check); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}

func TestRegister_ConfigValidation(t *testing.T) {
	t.Run("missing config key is rejected", func(t *testing.T) {
		cc := &CheckCollection{}
		check := stubCheck("TST_001", "test", "Test One", Pass)
		check.Configuration = map[string]*CheckConfiguration{
			"bad": {Key: "", Description: "desc", Default: 1},
		}
		if err := cc.Register(check); err == nil {
			t.Error("expected error for missing config key")
		}
	})

	t.Run("missing config description is rejected", func(t *testing.T) {
		cc := &CheckCollection{}
		check := stubCheck("TST_001", "test", "Test One", Pass)
		check.Configuration = map[string]*CheckConfiguration{
			"threshold": {Key: "threshold", Description: "", Default: 90},
		}
		if err := cc.Register(check); err == nil {
			t.Error("expected error for missing config description")
		}
	})

	t.Run("valid config registers and sets Check field", func(t *testing.T) {
		cc := &CheckCollection{}
		check := stubCheck("TST_001", "test", "Test One", Pass)
		check.Configuration = map[string]*CheckConfiguration{
			"threshold": {Key: "threshold", Description: "alert threshold", Default: 90},
		}
		if err := cc.Register(check); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		items := cc.ConfigurationItems()
		if len(items) != 1 || items[0].Check != "TST_001" {
			t.Errorf("expected config item with Check=TST_001, got %+v", items)
		}
	})
}

// --- SkipChecks / SkipSuites ---

func TestSkipChecks(t *testing.T) {
	cc := &CheckCollection{}
	cc.SkipChecks("TST_001", "TST_002")
	if len(cc.skipCheck) != 2 {
		t.Errorf("expected 2 skipped checks, got %d", len(cc.skipCheck))
	}

	t.Run("deduplicates", func(t *testing.T) {
		cc.SkipChecks("TST_001")
		if len(cc.skipCheck) != 2 {
			t.Errorf("expected still 2 after duplicate SkipChecks, got %d", len(cc.skipCheck))
		}
	})
}

func TestSkipSuites(t *testing.T) {
	cc := &CheckCollection{}
	cc.SkipSuites("cluster", "accounts")
	if len(cc.skipSuite) != 2 {
		t.Errorf("expected 2 skipped suites, got %d", len(cc.skipSuite))
	}

	t.Run("deduplicates", func(t *testing.T) {
		cc.SkipSuites("cluster")
		if len(cc.skipSuite) != 2 {
			t.Errorf("expected still 2 after duplicate SkipSuites, got %d", len(cc.skipSuite))
		}
	})
}

// --- EachCheck ---

func TestEachCheck_Ordering(t *testing.T) {
	cc := &CheckCollection{}
	// Register in scrambled order
	for _, c := range []Check{
		stubCheck("BETA_002", "beta", "Beta Two", Pass),
		stubCheck("ALPHA_001", "alpha", "Alpha One", Pass),
		stubCheck("BETA_001", "beta", "Beta One", Pass),
		stubCheck("ALPHA_002", "alpha", "Alpha Two", Pass),
	} {
		if err := cc.Register(c); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	var order []string
	cc.EachCheck(func(c *Check) { order = append(order, c.Code) })

	expected := []string{"ALPHA_001", "ALPHA_002", "BETA_001", "BETA_002"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], order[i])
		}
	}
}

// --- Run ---

func TestRun_Basic(t *testing.T) {
	cc := &CheckCollection{}
	if err := cc.Register(stubCheck("TST_001", "test", "Test One", Pass)); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	result := cc.Run(emptyArchive(t), 0, api.NewDefaultLogger(api.ErrorLevel))
	if result == nil {
		t.Fatal("Run returned nil")
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].Outcome != Pass {
		t.Errorf("expected Pass, got %v", result.Results[0].Outcome)
	}
	if result.Results[0].OutcomeString != "PASS" {
		t.Errorf("expected OutcomeString PASS, got %q", result.Results[0].OutcomeString)
	}
	if result.Outcomes["PASS"] != 1 {
		t.Errorf("expected Outcomes[PASS]=1, got %d", result.Outcomes["PASS"])
	}
}

func TestRun_DoesNotMutateSkipLists(t *testing.T) {
	// Add skip checks in reverse alphabetical order so that a sort would change the order.
	cc := &CheckCollection{}
	cc.SkipChecks("TST_Z", "TST_A")

	// Record original order before Run.
	origFirst := cc.skipCheck[0]

	cc.Run(emptyArchive(t), 0, api.NewDefaultLogger(api.ErrorLevel))

	// After Run the internal slice must be unchanged.
	if cc.skipCheck[0] != origFirst {
		t.Errorf("Run mutated skipCheck: first element changed from %q to %q", origFirst, cc.skipCheck[0])
	}
}

func TestRun_SkippedChecksAreSorted(t *testing.T) {
	cc := &CheckCollection{}
	cc.SkipChecks("TST_Z", "TST_A", "TST_M")

	result := cc.Run(emptyArchive(t), 0, api.NewDefaultLogger(api.ErrorLevel))

	expected := []string{"TST_A", "TST_M", "TST_Z"}
	if len(result.SkippedChecks) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, result.SkippedChecks)
	}
	for i, v := range expected {
		if result.SkippedChecks[i] != v {
			t.Errorf("position %d: expected %q, got %q", i, v, result.SkippedChecks[i])
		}
	}
}

func TestRun_SkippedCheckProducesSkipOutcome(t *testing.T) {
	cc := &CheckCollection{}
	if err := cc.Register(stubCheck("TST_001", "test", "Test One", Pass)); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	cc.SkipChecks("TST_001")

	result := cc.Run(emptyArchive(t), 0, api.NewDefaultLogger(api.ErrorLevel))
	if result.Results[0].Outcome != Skipped {
		t.Errorf("expected Skipped, got %v", result.Results[0].Outcome)
	}
	if result.Outcomes["SKIP"] != 1 {
		t.Errorf("expected Outcomes[SKIP]=1, got %d", result.Outcomes["SKIP"])
	}
}

func TestRun_SkippedSuiteProducesSkipOutcome(t *testing.T) {
	cc := &CheckCollection{}
	if err := cc.Register(stubCheck("TST_001", "test", "Test One", Pass)); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	cc.SkipSuites("test")

	result := cc.Run(emptyArchive(t), 0, api.NewDefaultLogger(api.ErrorLevel))
	if result.Results[0].Outcome != Skipped {
		t.Errorf("expected Skipped, got %v", result.Results[0].Outcome)
	}
}

func TestRun_ExamplesAlwaysNonNil(t *testing.T) {
	// A check that returns Pass with no examples must still have Examples.Examples as []string{}, not nil.
	cc := &CheckCollection{}
	if err := cc.Register(stubCheck("TST_001", "test", "Test One", Pass)); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	result := cc.Run(emptyArchive(t), 0, api.NewDefaultLogger(api.ErrorLevel))
	if result.Results[0].Examples.Examples == nil {
		t.Error("Examples.Examples must not be nil when check returns no examples")
	}
}

func TestRun_MetadataMissingDoesNotFail(t *testing.T) {
	// An archive with no metadata artifact must not cause Run to fail; Metadata stays zero.
	cc := &CheckCollection{}
	if err := cc.Register(stubCheck("TST_001", "test", "Test One", Pass)); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	result := cc.Run(emptyArchive(t), 0, api.NewDefaultLogger(api.ErrorLevel))
	if result == nil {
		t.Fatal("Run returned nil")
	}
	if result.Metadata.ConnectURL != "" {
		t.Errorf("expected zero Metadata, got ConnectURL=%q", result.Metadata.ConnectURL)
	}
}

func TestRun_ResultType(t *testing.T) {
	cc := &CheckCollection{}
	result := cc.Run(emptyArchive(t), 0, api.NewDefaultLogger(api.ErrorLevel))
	if result.Type != "io.nats.audit.v1.analysis" {
		t.Errorf("expected type io.nats.audit.v1.analysis, got %q", result.Type)
	}
}

// --- Outcome.String ---

func TestOutcomeString(t *testing.T) {
	cases := []struct {
		outcome Outcome
		want    string
	}{
		{Pass, "PASS"},
		{PassWithIssues, "WARN"},
		{Fail, "FAIL"},
		{Skipped, "SKIP"},
	}
	for _, tc := range cases {
		if got := tc.outcome.String(); got != tc.want {
			t.Errorf("Outcome(%d).String() = %q, want %q", tc.outcome, got, tc.want)
		}
	}
}

// --- configItemKey ---

func TestConfigItemKey_LowercasesBoth(t *testing.T) {
	cases := []struct {
		code, key, want string
	}{
		{"ACCOUNTS_001", "connections", "accounts_001_connections"},
		{"ACCOUNTS_001", "Connections", "accounts_001_connections"},
		{"accounts_001", "CONNECTIONS", "accounts_001_connections"},
	}
	for _, tc := range cases {
		got := configItemKey(tc.code, tc.key)
		if got != tc.want {
			t.Errorf("configItemKey(%q, %q) = %q, want %q", tc.code, tc.key, got, tc.want)
		}
	}
}

// --- NewCollection ---

func TestNewCollection(t *testing.T) {
	cc := NewCollection()
	if cc == nil {
		t.Fatal("NewCollection returned nil")
	}
	var count int
	cc.EachCheck(func(_ *Check) { count++ })
	if count != 0 {
		t.Errorf("expected empty collection, got %d checks", count)
	}
}

// --- MustRegister ---

func TestMustRegister_PanicsOnInvalidCheck(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid check, got none")
		}
	}()

	cc := &CheckCollection{}
	bad := stubCheck("TST_001", "test", "Test One", Pass)
	bad.Code = ""
	cc.MustRegister(bad)
}

// --- NewDefaultCheckCollection ---

func TestNewDefaultCheckCollection(t *testing.T) {
	cc, err := NewDefaultCheckCollection()
	if err != nil {
		t.Fatalf("NewDefaultCheckCollection failed: %v", err)
	}

	var count int
	cc.EachCheck(func(_ *Check) { count++ })
	if count == 0 {
		t.Error("expected default collection to contain checks")
	}
}
