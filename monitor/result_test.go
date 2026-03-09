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
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func requireEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func requireContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("%q does not contain %q", s, sub)
	}
}

func requireNotContains(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("%q should not contain %q", s, sub)
	}
}


func TestResultPrepare(t *testing.T) {
	t.Run("criticals wins over warnings", func(t *testing.T) {
		r := &Result{}
		r.Critical("bad thing")
		r.Warn("iffy thing")
		r.prepare()
		if r.Status != CriticalStatus {
			t.Errorf("expected Critical, got %s", r.Status)
		}
	})

	t.Run("warnings only", func(t *testing.T) {
		r := &Result{}
		r.Warn("iffy thing")
		r.prepare()
		if r.Status != WarningStatus {
			t.Errorf("expected Warning, got %s", r.Status)
		}
	})

	t.Run("no issues gives OK", func(t *testing.T) {
		r := &Result{}
		r.prepare()
		if r.Status != OKStatus {
			t.Errorf("expected OK, got %s", r.Status)
		}
	})

	t.Run("nil PerfData is initialised", func(t *testing.T) {
		r := &Result{}
		r.prepare()
		if r.PerfData == nil {
			t.Error("PerfData should not be nil after prepare")
		}
	})
}


func TestResultOkIfNoWarningsOrCriticals(t *testing.T) {
	t.Run("adds ok when clean", func(t *testing.T) {
		r := &Result{}
		r.OkIfNoWarningsOrCriticals("all good")
		requireLen(t, r.OKs, 1)
		requireEqual(t, r.OKs[0], "all good")
	})

	t.Run("no ok when warnings present", func(t *testing.T) {
		r := &Result{}
		r.Warn("iffy")
		r.OkIfNoWarningsOrCriticals("all good")
		requireEmpty(t, r.OKs)
	})

	t.Run("no ok when criticals present", func(t *testing.T) {
		r := &Result{}
		r.Critical("bad")
		r.OkIfNoWarningsOrCriticals("all good")
		requireEmpty(t, r.OKs)
	})

	t.Run("formatted variant", func(t *testing.T) {
		r := &Result{}
		r.OkIfNoWarningsOrCriticalsf("count=%d", 42)
		requireLen(t, r.OKs, 1)
		requireEqual(t, r.OKs[0], "count=42")
	})
}


func TestResultCriticalIfErr(t *testing.T) {
	t.Run("nil error is no-op", func(t *testing.T) {
		r := &Result{}
		if r.CriticalIfErr(nil, "should not appear") {
			t.Error("expected false for nil error")
		}
		requireEmpty(t, r.Criticals)
	})

	t.Run("non-nil error appends critical", func(t *testing.T) {
		r := &Result{}
		if !r.CriticalIfErr(errors.New("boom"), "something failed") {
			t.Error("expected true for non-nil error")
		}
		requireLen(t, r.Criticals, 1)
		requireEqual(t, r.Criticals[0], "something failed")
	})

	t.Run("formatted nil is no-op", func(t *testing.T) {
		r := &Result{}
		if r.CriticalIfErrf(nil, "fmt %s", "nope") {
			t.Error("expected false")
		}
		requireEmpty(t, r.Criticals)
	})

	t.Run("formatted non-nil appends critical", func(t *testing.T) {
		r := &Result{}
		if !r.CriticalIfErrf(errors.New("e"), "error: %v", errors.New("e")) {
			t.Error("expected true")
		}
		requireLen(t, r.Criticals, 1)
		requireContains(t, r.Criticals[0], "error:")
	})
}


func TestResultExitCode(t *testing.T) {
	tests := []struct {
		name   string
		format RenderFormat
		status Status
		want   int
	}{
		{"prometheus always 0 even on critical", PrometheusFormat, CriticalStatus, 0},
		{"nagios ok", NagiosFormat, OKStatus, 0},
		{"nagios warning", NagiosFormat, WarningStatus, 1},
		{"nagios critical", NagiosFormat, CriticalStatus, 2},
		{"nagios unknown", NagiosFormat, UnknownStatus, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{RenderFormat: tt.format, Status: tt.status}
			got := r.exitCode()
			if got != tt.want {
				t.Errorf("exitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}


func TestResultRenderNagios(t *testing.T) {
	t.Run("ok with no perf data", func(t *testing.T) {
		r := &Result{Name: "mycheck"}
		r.Ok("things look fine")
		r.prepare()
		out := r.renderNagios()
		requireContains(t, out, "OK:things look fine")
		requireNotContains(t, out, "|")
	})

	t.Run("critical message included", func(t *testing.T) {
		r := &Result{Name: "mycheck"}
		r.Critical("disk full")
		r.prepare()
		out := r.renderNagios()
		requireContains(t, out, "Crit:disk full")
	})

	t.Run("warning message included", func(t *testing.T) {
		r := &Result{Name: "mycheck"}
		r.Warn("disk filling up")
		r.prepare()
		out := r.renderNagios()
		requireContains(t, out, "Warn:disk filling up")
	})

	t.Run("perf data appended after pipe", func(t *testing.T) {
		r := &Result{Name: "mycheck"}
		r.Ok("fine")
		r.Pd(&PerfDataItem{Name: "msgs", Value: 42})
		r.prepare()
		out := r.renderNagios()
		requireContains(t, out, "| msgs=42")
	})

	t.Run("Output overrides OKs in nagios output", func(t *testing.T) {
		r := &Result{Name: "mycheck", Output: "custom output"}
		r.Ok("this should not appear")
		r.prepare()
		out := r.renderNagios()
		requireContains(t, out, "custom output")
		requireNotContains(t, out, "OK:this should not appear")
	})
}


func TestResultRenderJSON(t *testing.T) {
	t.Run("valid json output", func(t *testing.T) {
		r := &Result{Name: "mycheck", Check: "suite"}
		r.Critical("something broke")
		r.prepare()
		out := r.renderJSON()

		var decoded Result
		if err := json.Unmarshal([]byte(out), &decoded); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
		}
		requireEqual(t, decoded.Name, "mycheck")
		requireLen(t, decoded.Criticals, 1)
		requireEqual(t, decoded.Criticals[0], "something broke")
	})

	t.Run("status field present", func(t *testing.T) {
		r := &Result{Name: "mycheck"}
		r.Ok("fine")
		r.prepare()
		out := r.renderJSON()
		requireContains(t, out, `"status"`)
	})
}


func TestResultRenderHuman(t *testing.T) {
	t.Run("contains name and ok message", func(t *testing.T) {
		r := &Result{Name: "mycheck"}
		r.Ok("all fine")
		r.prepare()
		out := r.renderHuman()
		requireContains(t, out, "mycheck")
		requireContains(t, out, "all fine")
	})

	t.Run("perf data table shown when present", func(t *testing.T) {
		r := &Result{Name: "mycheck"}
		r.Ok("fine")
		r.Pd(&PerfDataItem{Name: "msgs", Value: 100, Help: "message count"})
		r.prepare()
		out := r.renderHuman()
		requireContains(t, out, "msgs")
		requireContains(t, out, "message count")
	})

	t.Run("no perf data table when empty", func(t *testing.T) {
		r := &Result{Name: "mycheck"}
		r.Ok("fine")
		r.prepare()
		out := r.renderHuman()
		requireNotContains(t, out, "Check Metrics")
	})
}


func TestResultRenderPrometheus(t *testing.T) {
	t.Run("contains status gauge", func(t *testing.T) {
		r := &Result{Name: "mycheck", Check: "suite", NameSpace: "nats"}
		r.Ok("fine")
		r.prepare()
		out := r.renderPrometheus()
		requireContains(t, out, "nats_suite_status_code")
	})

	t.Run("contains perf data gauge", func(t *testing.T) {
		r := &Result{Name: "mycheck", Check: "suite", NameSpace: "nats"}
		r.Pd(&PerfDataItem{Name: "msgs", Value: 77, Help: "message count"})
		r.prepare()
		out := r.renderPrometheus()
		requireContains(t, out, "nats_suite_msgs")
		requireContains(t, out, "77")
	})

	t.Run("does not mutate global prometheus registerer", func(t *testing.T) {
		before := prometheus.DefaultRegisterer
		r := &Result{Name: "mycheck", Check: "suite"}
		r.prepare()
		r.renderPrometheus()
		if prometheus.DefaultRegisterer != before {
			t.Error("renderPrometheus mutated the global Prometheus DefaultRegisterer")
		}
	})

	t.Run("concurrent calls do not panic or race", func(t *testing.T) {
		done := make(chan struct{}, 5)
		for i := 0; i < 5; i++ {
			go func(i int) {
				defer func() { done <- struct{}{} }()
				r := &Result{Name: fmt.Sprintf("check%d", i), Check: fmt.Sprintf("suite%d", i)}
				r.Ok("fine")
				r.prepare()
				r.renderPrometheus()
			}(i)
		}
		for i := 0; i < 5; i++ {
			<-done
		}
	})
}


func TestResultGenericExitOutFile(t *testing.T) {
	t.Run("writes result to file atomically with correct permissions", func(t *testing.T) {
		dir := t.TempDir()
		outFile := filepath.Join(dir, "result.txt")

		r := &Result{Name: "mycheck"}
		r.Ok("all systems go")
		r.prepare()
		content := r.String()

		f, err := os.CreateTemp(filepath.Dir(outFile), "")
		if err != nil {
			t.Fatal(err)
		}
		tmpName := f.Name()

		_, err = fmt.Fprintln(f, content)
		if err != nil {
			f.Close()
			os.Remove(tmpName)
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			os.Remove(tmpName)
			t.Fatal(err)
		}
		if err = os.Chmod(tmpName, 0600); err != nil {
			os.Remove(tmpName)
			t.Fatal(err)
		}
		if err = os.Rename(tmpName, outFile); err != nil {
			os.Remove(tmpName)
			t.Fatal(err)
		}

		// Temp file must be gone after successful rename.
		if _, err := os.Stat(tmpName); !os.IsNotExist(err) {
			t.Errorf("temp file %s should have been moved away", tmpName)
		}

		data, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("output file not readable: %v", err)
		}
		if !strings.Contains(string(data), "all systems go") {
			t.Errorf("output file missing expected content: %s", data)
		}

		info, err := os.Stat(outFile)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected mode 0600, got %o", info.Mode().Perm())
		}
	})
}


func TestHumanizeDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{500 * time.Microsecond, "500µs"},
		{5 * time.Millisecond, "5ms"},
		{time.Duration(math.MaxInt64), "never"},
		{90 * time.Second, "1m30s"},
		{3661 * time.Second, "1h1m1s"},
		{25 * time.Hour, "1d1h0m0s"},
		{400 * 24 * time.Hour, "1y35d0h0m0s"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := humanizeDuration(tt.input)
			if got != tt.want {
				t.Errorf("humanizeDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}


func TestF(t *testing.T) {
	t.Run("string slice joined", func(t *testing.T) {
		requireEqual(t, f([]string{"a", "b", "c"}), "a, b, c")
	})

	t.Run("time.Duration", func(t *testing.T) {
		requireEqual(t, f(5*time.Millisecond), "5ms")
	})

	t.Run("time.Time", func(t *testing.T) {
		ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
		got := f(ts.Local())
		requireContains(t, got, "2025-01-02")
	})

	t.Run("bool", func(t *testing.T) {
		requireEqual(t, f(true), "true")
		requireEqual(t, f(false), "false")
	})

	t.Run("uint commas", func(t *testing.T) {
		requireEqual(t, f(uint(1000000)), "1,000,000")
	})

	t.Run("uint32 commas", func(t *testing.T) {
		requireEqual(t, f(uint32(1000)), "1,000")
	})

	t.Run("uint16 commas", func(t *testing.T) {
		requireEqual(t, f(uint16(1000)), "1,000")
	})

	t.Run("uint64 commas within int64 range", func(t *testing.T) {
		requireEqual(t, f(uint64(1000000)), "1,000,000")
	})

	t.Run("uint64 above int64 max does not overflow", func(t *testing.T) {
		big := uint64(math.MaxInt64) + 1
		got := f(big)
		if strings.HasPrefix(got, "-") {
			t.Errorf("uint64 overflow: f(%d) = %q", big, got)
		}
		requireEqual(t, got, fmt.Sprintf("%d", big))
	})

	t.Run("int commas", func(t *testing.T) {
		requireEqual(t, f(int(1000)), "1,000")
	})

	t.Run("int32 commas", func(t *testing.T) {
		requireEqual(t, f(int32(1000)), "1,000")
	})

	t.Run("int64 commas", func(t *testing.T) {
		requireEqual(t, f(int64(1000000)), "1,000,000")
	})

	t.Run("float32", func(t *testing.T) {
		got := f(float32(3.14))
		requireContains(t, got, "3.14")
	})

	t.Run("float64", func(t *testing.T) {
		got := f(float64(1234.5))
		requireContains(t, got, "1,234.5")
	})

	t.Run("unknown type falls back to Sprintf", func(t *testing.T) {
		type myType struct{ x int }
		got := f(myType{42})
		requireContains(t, got, "42")
	})
}