// Copyright 2026 The NATS Authors
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

package monitor

import (
	"testing"
)

func TestPerfDataItemString(t *testing.T) {
	tests := []struct {
		name     string
		item     PerfDataItem
		expected string
	}{
		{
			name:     "value only",
			item:     PerfDataItem{Name: "msgs", Value: 42},
			expected: "msgs=42",
		},
		{
			name:     "value with unit",
			item:     PerfDataItem{Name: "size", Value: 1024, Unit: "B"},
			expected: "size=1024B",
		},
		{
			name:     "seconds unit uses four decimals",
			item:     PerfDataItem{Name: "rtt", Value: 0.0012, Unit: "s"},
			expected: "rtt=0.0012s",
		},
		{
			name:     "warn and crit",
			item:     PerfDataItem{Name: "peers", Value: 3, Warn: 3, Crit: 3},
			expected: "peers=3;3;3",
		},
		{
			name:     "warn only",
			item:     PerfDataItem{Name: "peers", Value: 3, Warn: 3},
			expected: "peers=3;3",
		},
		{
			name:     "crit only — empty warn slot",
			item:     PerfDataItem{Name: "peers", Value: 3, Crit: 3},
			expected: "peers=3;;3",
		},
		{
			name:     "negative warn with zero crit — was silently dropped before fix",
			item:     PerfDataItem{Name: "delta", Value: 5, Warn: -1},
			expected: "delta=5;-1",
		},
		{
			name:     "negative warn with positive crit",
			item:     PerfDataItem{Name: "delta", Value: 5, Warn: -1, Crit: 10},
			expected: "delta=5;-1;10",
		},
		{
			name:     "zero value",
			item:     PerfDataItem{Name: "lag", Value: 0},
			expected: "lag=0",
		},
		{
			name:     "name with spaces is quoted",
			item:     PerfDataItem{Name: "peer lag", Value: 7},
			expected: "'peer lag'=7",
		},
		{
			name:     "name with tab is quoted",
			item:     PerfDataItem{Name: "peer\tlag", Value: 7},
			expected: "'peer\tlag'=7",
		},
		{
			name:     "name without spaces is not quoted",
			item:     PerfDataItem{Name: "peer_lag", Value: 7},
			expected: "peer_lag=7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.item.String()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPerfDataString(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		pd := PerfData{}
		if got := pd.String(); got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("single item", func(t *testing.T) {
		pd := PerfData{
			{Name: "msgs", Value: 1},
		}
		if got := pd.String(); got != "msgs=1" {
			t.Errorf("got %q, want %q", got, "msgs=1")
		}
	})

	t.Run("multiple items space joined", func(t *testing.T) {
		pd := PerfData{
			{Name: "msgs", Value: 1},
			{Name: "bytes", Value: 512, Unit: "B"},
			{Name: "rtt", Value: 0.001, Unit: "s"},
		}
		want := "msgs=1 bytes=512B rtt=0.0010s"
		if got := pd.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
