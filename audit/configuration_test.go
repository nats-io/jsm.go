package audit

import (
	"math"
	"strings"
	"testing"
)

// --- Value ---

func TestCheckConfigurationValue(t *testing.T) {
	t.Run("returns default when SetValue is nil", func(t *testing.T) {
		c := &CheckConfiguration{Default: 42}
		if got := c.Value(); got != 42 {
			t.Errorf("expected 42, got %v", got)
		}
	})

	t.Run("returns SetValue when set", func(t *testing.T) {
		v := 99.0
		c := &CheckConfiguration{Default: 42, SetValue: &v}
		if got := c.Value(); got != 99 {
			t.Errorf("expected 99, got %v", got)
		}
	})

	t.Run("nil receiver returns 0", func(t *testing.T) {
		var c *CheckConfiguration
		if got := c.Value(); got != 0 {
			t.Errorf("expected 0 for nil receiver, got %v", got)
		}
	})
}

// --- String ---

func TestCheckConfigurationString(t *testing.T) {
	t.Run("returns formatted value", func(t *testing.T) {
		c := &CheckConfiguration{Default: 1234567.89}
		got := c.String()
		if got == "" {
			t.Error("expected non-empty string")
		}
		// humanize.Commaf adds commas: "1,234,567.89"
		if !strings.Contains(got, "1,234") {
			t.Errorf("expected comma-formatted number, got %q", got)
		}
	})

	t.Run("nil receiver returns empty string", func(t *testing.T) {
		var c *CheckConfiguration
		if got := c.String(); got != "" {
			t.Errorf("expected empty string for nil receiver, got %q", got)
		}
	})
}

// --- Set: PercentageUnit ---

func TestCheckConfigurationSet_Percentage(t *testing.T) {
	t.Run("accepts plain number", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		if err := c.Set("90"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != 90 {
			t.Errorf("expected 90, got %v", c.Value())
		}
	})

	t.Run("strips single trailing percent sign", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		if err := c.Set("75%"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != 75 {
			t.Errorf("expected 75, got %v", c.Value())
		}
	})

	t.Run("double percent sign is rejected (TrimSuffix not TrimRight)", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		if err := c.Set("75%%"); err == nil {
			t.Error("expected error for double %%, got nil")
		}
	})

	t.Run("accepts boundary 0", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		if err := c.Set("0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != 0 {
			t.Errorf("expected 0, got %v", c.Value())
		}
	})

	t.Run("accepts boundary 100", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		if err := c.Set("100"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != 100 {
			t.Errorf("expected 100, got %v", c.Value())
		}
	})

	t.Run("rejects negative value", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		if err := c.Set("-1"); err == nil {
			t.Error("expected error for negative value")
		}
	})

	t.Run("rejects value over 100", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		if err := c.Set("101"); err == nil {
			t.Error("expected error for value > 100")
		}
	})

	t.Run("rejects non-numeric input", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		if err := c.Set("abc"); err == nil {
			t.Error("expected error for non-numeric input")
		}
	})

	t.Run("error message says non-negative", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit}
		err := c.Set("-1")
		if err == nil || !strings.Contains(err.Error(), "non-negative") {
			t.Errorf("expected 'non-negative' in error, got: %v", err)
		}
	})
}

// --- Set: UIntUnit ---

func TestCheckConfigurationSet_UInt(t *testing.T) {
	t.Run("accepts valid whole number", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit}
		if err := c.Set("1000"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != 1000 {
			t.Errorf("expected 1000, got %v", c.Value())
		}
	})

	t.Run("accepts zero", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit}
		if err := c.Set("0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != 0 {
			t.Errorf("expected 0, got %v", c.Value())
		}
	})

	t.Run("rejects negative value", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit}
		if err := c.Set("-1"); err == nil {
			t.Error("expected error for negative value")
		}
	})

	t.Run("rejects fractional value", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit}
		if err := c.Set("1.5"); err == nil {
			t.Error("expected error for fractional value")
		}
	})

	t.Run("rejects non-numeric input", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit}
		if err := c.Set("abc"); err == nil {
			t.Error("expected error for non-numeric input")
		}
	})

	t.Run("error message says non-negative", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit}
		err := c.Set("-1")
		if err == nil || !strings.Contains(err.Error(), "non-negative") {
			t.Errorf("expected 'non-negative' in error, got: %v", err)
		}
	})
}

// --- Set: IntUnit ---

func TestCheckConfigurationSet_Int(t *testing.T) {
	t.Run("accepts positive whole number", func(t *testing.T) {
		c := &CheckConfiguration{Unit: IntUnit}
		if err := c.Set("500"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != 500 {
			t.Errorf("expected 500, got %v", c.Value())
		}
	})

	t.Run("accepts negative whole number", func(t *testing.T) {
		c := &CheckConfiguration{Unit: IntUnit}
		if err := c.Set("-500"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != -500 {
			t.Errorf("expected -500, got %v", c.Value())
		}
	})

	t.Run("accepts zero", func(t *testing.T) {
		c := &CheckConfiguration{Unit: IntUnit}
		if err := c.Set("0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects fractional value", func(t *testing.T) {
		c := &CheckConfiguration{Unit: IntUnit}
		if err := c.Set("1.5"); err == nil {
			t.Error("expected error for fractional value")
		}
	})

	t.Run("rejects non-numeric input", func(t *testing.T) {
		c := &CheckConfiguration{Unit: IntUnit}
		if err := c.Set("abc"); err == nil {
			t.Error("expected error for non-numeric input")
		}
	})
}

// --- Set: no unit (empty string) ---

func TestCheckConfigurationSet_NoUnit(t *testing.T) {
	t.Run("accepts any float when unit is empty", func(t *testing.T) {
		c := &CheckConfiguration{Unit: ""}
		if err := c.Set("1.5"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Value() != 1.5 {
			t.Errorf("expected 1.5, got %v", c.Value())
		}
	})

	t.Run("rejects non-numeric input even with no unit", func(t *testing.T) {
		c := &CheckConfiguration{Unit: ""}
		if err := c.Set("abc"); err == nil {
			t.Error("expected error for non-numeric input")
		}
	})
}

// --- Set: unknown unit ---

func TestCheckConfigurationSet_UnknownUnit(t *testing.T) {
	t.Run("non-empty unknown unit returns error", func(t *testing.T) {
		c := &CheckConfiguration{Unit: "bogus"}
		if err := c.Set("50"); err == nil {
			t.Error("expected error for unknown unit")
		}
	})
}

// --- validateDefault ---

func TestValidateDefault(t *testing.T) {
	t.Run("percentage: valid default passes", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit, Default: 90}
		if err := c.validateDefault(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("percentage: negative default is rejected", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit, Default: -1}
		if err := c.validateDefault(); err == nil {
			t.Error("expected error for negative percentage default")
		}
	})

	t.Run("percentage: default over 100 is rejected", func(t *testing.T) {
		c := &CheckConfiguration{Unit: PercentageUnit, Default: 101}
		if err := c.validateDefault(); err == nil {
			t.Error("expected error for percentage default > 100")
		}
	})

	t.Run("uint: valid default passes", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit, Default: 1000}
		if err := c.validateDefault(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("uint: negative default is rejected", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit, Default: -1}
		if err := c.validateDefault(); err == nil {
			t.Error("expected error for negative uint default")
		}
	})

	t.Run("uint: fractional default is rejected", func(t *testing.T) {
		c := &CheckConfiguration{Unit: UIntUnit, Default: 1.5}
		if err := c.validateDefault(); err == nil {
			t.Error("expected error for fractional uint default")
		}
	})

	t.Run("int: valid default passes", func(t *testing.T) {
		c := &CheckConfiguration{Unit: IntUnit, Default: -500}
		if err := c.validateDefault(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("int: fractional default is rejected", func(t *testing.T) {
		c := &CheckConfiguration{Unit: IntUnit, Default: math.Pi}
		if err := c.validateDefault(); err == nil {
			t.Error("expected error for fractional int default")
		}
	})

	t.Run("no unit: any default passes", func(t *testing.T) {
		c := &CheckConfiguration{Unit: "", Default: 1.5}
		if err := c.validateDefault(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- validateDefault via Register ---

func TestRegister_RejectsInvalidDefaults(t *testing.T) {
	cases := []struct {
		name string
		cfg  *CheckConfiguration
	}{
		{"negative percentage default", &CheckConfiguration{Key: "k", Description: "d", Unit: PercentageUnit, Default: -1}},
		{"over-100 percentage default", &CheckConfiguration{Key: "k", Description: "d", Unit: PercentageUnit, Default: 101}},
		{"negative uint default", &CheckConfiguration{Key: "k", Description: "d", Unit: UIntUnit, Default: -1}},
		{"fractional uint default", &CheckConfiguration{Key: "k", Description: "d", Unit: UIntUnit, Default: 1.5}},
		{"fractional int default", &CheckConfiguration{Key: "k", Description: "d", Unit: IntUnit, Default: 1.5}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cc := &CheckCollection{}
			check := stubCheck("TST_001", "test", "Test One", Pass)
			check.Configuration = map[string]*CheckConfiguration{"k": tc.cfg}
			if err := cc.Register(check); err == nil {
				t.Errorf("expected Register to reject config with %s", tc.name)
			}
		})
	}
}