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
	"fmt"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
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

// Value retrieves the set value or default value
func (c *CheckConfiguration) Value() float64 {
	if c.SetValue != nil {
		return *c.SetValue
	}

	return c.Default
}

func (c *CheckConfiguration) String() string {
	return humanize.Commaf(c.Value())
}

// Set supports fisk
func (c *CheckConfiguration) Set(v string) error {
	var f float64
	var err error

	if c.Unit == PercentageUnit {
		f, err = strconv.ParseFloat(strings.TrimRight(v, "%"), 64)
		if err != nil {
			return err
		}
		if f < 0 {
			return fmt.Errorf("percentage values must be positive")
		}
		if f > 100 {
			return fmt.Errorf("percentage values may not exceed 100")
		}

		if f > 1 {
			f = f / 100
		}
	} else {
		f, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}

		if c.Unit == UIntUnit {
			if f < 0 {
				return fmt.Errorf("value must be positive")
			}
		}
	}

	c.SetValue = &f

	return nil
}

// SetVal supports fisk
//func (c *CheckConfiguration) SetVal(s fisk.Settings) {
//	s.SetValue(c)
//}
