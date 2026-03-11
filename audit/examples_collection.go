// Copyright 2024-2026 The NATS Authors
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
	"strings"
)

// ExamplesCollection stores examples of issues found by a check as it scans entities in an archive.
// A limit can be passed to avoid accumulating hundreds of examples.
// After the limit is reached, further examples are just counted but not stored.
type ExamplesCollection struct {
	Examples []string `json:"examples,omitempty"`
	Error    string   `json:"error,omitempty"`
	Limit    uint     `json:"-"`
	overflow int
}

// newExamplesCollection creates a new empty collection of examples.
// Use 0 as limit to store unlimited examples.
func newExamplesCollection(limit uint) *ExamplesCollection {
	return &ExamplesCollection{
		Limit:    limit,
		Examples: []string{},
	}
}

// Add adds an example issue to the collection.
// If a non-zero Limit is set and already reached, the example is counted but not stored.
func (c *ExamplesCollection) Add(format string, a ...any) {
	if c == nil {
		return
	}
	if c.Limit > 0 && len(c.Examples) >= int(c.Limit) {
		c.overflow++
		return
	}
	c.Examples = append(c.Examples, fmt.Sprintf(format, a...))
}

// Clear removes all stored examples and resets the overflow counter.
func (c *ExamplesCollection) Clear() {
	if c == nil {
		return
	}
	c.Examples = []string{}
	c.overflow = 0
}

// Count returns the total number of examples added to this collection, including those not stored due to the limit.
func (c *ExamplesCollection) Count() int {
	if c == nil {
		return 0
	}

	return len(c.Examples) + c.overflow
}

// String produces a multi-line string with one example per line.
// If more examples were added than the limit, an extra line is printed with the number of omitted examples.
func (c *ExamplesCollection) String() string {
	if c == nil {
		return ""
	}

	b := &strings.Builder{}
	for _, example := range c.Examples {
		b.WriteString(fmt.Sprintf(" - %s\n", example))
	}

	if c.overflow > 0 {
		b.WriteString(fmt.Sprintf(" - ... and %d more ...\n", c.overflow))
	}

	return b.String()
}
