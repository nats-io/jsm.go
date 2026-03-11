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
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/choria-io/fisk"
)

// CheckConfiguration describes and holds the configuration for a check
type CheckConfiguration struct {
	Key         string            `json:"key"`
	Check       string            `json:"check"`
	Description string            `json:"description"`
	Default     float64           `json:"default"`
	Unit        ConfigurationUnit `json:"unit"`
	SetValue    *float64          `json:"set_value,omitempty"`
}

type ConfigurationUnit string

const (
	PercentageUnit ConfigurationUnit = "%"
	IntUnit        ConfigurationUnit = "int"
	UIntUnit       ConfigurationUnit = "uint"
)

// Value retrieves the set value or the default value.
// Safe to call on a nil receiver; returns 0 in that case.
func (c *CheckConfiguration) Value() float64 {
	if c == nil {
		return 0
	}
	if c.SetValue != nil {
		return *c.SetValue
	}
	return c.Default
}

// String returns a human-readable representation of the current value.
// Safe to call on a nil receiver; returns "" in that case.
func (c *CheckConfiguration) String() string {
	if c == nil {
		return ""
	}
	return humanize.Commaf(c.Value())
}

// validateUnitValue checks that f satisfies the numeric constraints for unit.
// Returns nil for an empty unit (no constraint) and for unrecognized units.
func validateUnitValue(unit ConfigurationUnit, f float64) error {
	switch unit {
	case PercentageUnit:
		if f < 0 {
			return fmt.Errorf("percentage values must be non-negative")
		}
		if f > 100 {
			return fmt.Errorf("percentage values may not exceed 100")
		}
	case UIntUnit:
		if f < 0 {
			return fmt.Errorf("value must be non-negative")
		}
		if f != math.Trunc(f) {
			return fmt.Errorf("value must be a whole number")
		}
	case IntUnit:
		if f != math.Trunc(f) {
			return fmt.Errorf("value must be a whole number")
		}
	}
	return nil
}

// Set parses v, validates it against the unit constraints, and stores the result.
// Supports fisk.
func (c *CheckConfiguration) Set(v string) error {
	// Reject non-empty unit values that are not recognized.
	switch c.Unit {
	case PercentageUnit, UIntUnit, IntUnit, "":
	default:
		return fmt.Errorf("unknown configuration unit %q", c.Unit)
	}

	// Strip the optional "%" suffix only for percentage values.
	input := v
	if c.Unit == PercentageUnit {
		input = strings.TrimSuffix(v, "%")
	}

	f, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return err
	}

	if err := validateUnitValue(c.Unit, f); err != nil {
		return err
	}

	c.SetValue = &f
	return nil
}

// SetVal supports fisk
func (c *CheckConfiguration) SetVal(s fisk.Settings) {
	s.SetValue(c)
}

// validateDefault checks that the Default value satisfies the unit's own constraints.
// It is called by Register to catch misconfigured check definitions early.
func (c *CheckConfiguration) validateDefault() error {
	return validateUnitValue(c.Unit, c.Default)
}
