// Copyright 2023 The NATS Authors
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
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

type RenderFormat int

const (
	NagiosFormat RenderFormat = iota
	PrometheusFormat
	TextFormat
	JSONFormat
)

type Result struct {
	Output       string       `json:"output,omitempty"`
	Status       Status       `json:"status"`
	Check        string       `json:"check_suite"`
	Name         string       `json:"check_name"`
	Warnings     []string     `json:"warning,omitempty"`
	Criticals    []string     `json:"critical,omitempty"`
	OKs          []string     `json:"ok,omitempty"`
	PerfData     PerfData     `json:"perf_data"`
	RenderFormat RenderFormat `json:"-"`
	NameSpace    string       `json:"-"`
	OutFile      string       `json:"-"`
	Trace        bool         `json:"-"`
}

func (r *Result) Pd(pd ...*PerfDataItem) {
	r.PerfData = append(r.PerfData, pd...)
}

func (r *Result) CriticalExit(format string, a ...any) {
	r.Critical(format, a...)
	r.GenericExit()
}

func (r *Result) Critical(format string, a ...any) {
	r.Criticals = append(r.Criticals, fmt.Sprintf(format, a...))
}

func (r *Result) Warn(format string, a ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, a...))
}

func (r *Result) Ok(format string, a ...any) {
	r.OKs = append(r.OKs, fmt.Sprintf(format, a...))
}

func (r *Result) OkIfNoWarningsOrCriticals(format string, a ...any) {
	if len(r.Warnings) == 0 && len(r.Criticals) == 0 {
		r.Ok(format, a...)
	}
}

func (r *Result) CriticalExitIfErr(err error, format string, a ...any) bool {
	if err == nil {
		return false
	}

	r.CriticalExit(format, a...)

	return true
}

func (r *Result) CriticalIfErr(err error, format string, a ...any) bool {
	if err == nil {
		return false
	}

	r.Critical(format, a...)

	return true
}

func (r *Result) nagiosCode() int {
	switch r.Status {
	case OKStatus:
		return 0
	case WarningStatus:
		return 1
	case CriticalStatus:
		return 2
	default:
		return 3
	}
}

func (r *Result) exitCode() int {
	if r.RenderFormat == PrometheusFormat {
		return 0
	}

	return r.nagiosCode()
}

func (r *Result) Exit() {
	os.Exit(r.exitCode())
}

func (r *Result) renderHuman() string {
	buf := bytes.NewBuffer([]byte{})

	fmt.Fprintf(buf, "%s: %s\n\n", r.Name, r.Status)

	tblWriter := newTableWriter("")
	tblWriter.AppendHeader(table.Row{"Status", "Message"})
	lines := 0
	for _, ok := range r.OKs {
		tblWriter.AppendRow(table.Row{"OK", ok})
		lines++
	}
	for _, warn := range r.Warnings {
		tblWriter.AppendRow(table.Row{"Warning", warn})
		lines++
	}
	for _, crit := range r.Criticals {
		tblWriter.AppendRow(table.Row{"Critical", crit})
		lines++
	}

	if lines > 0 {
		fmt.Fprintln(buf, "Status Detail")
		fmt.Fprintln(buf)
		fmt.Fprint(buf, tblWriter.Render())
		fmt.Fprintln(buf)
	}

	tblWriter = newTableWriter("")
	tblWriter.AppendHeader(table.Row{"Metric", "Value", "Unit", "Critical Threshold", "Warning Threshold", "Description"})
	lines = 0
	for _, pd := range r.PerfData {
		tblWriter.AppendRow(table.Row{pd.Name, f(pd.Value), pd.Unit, f(pd.Crit), f(pd.Warn), pd.Help})
		lines++
	}
	if lines > 0 {
		fmt.Fprintln(buf)
		fmt.Fprintln(buf, "Check Metrics")
		fmt.Fprintln(buf)
		fmt.Fprint(buf, tblWriter.Render())
		fmt.Fprintln(buf)
	}

	return buf.String()
}

func (r *Result) Collect(ch chan<- prometheus.Metric) {
	r.prepare()

	for _, c := range append(r.perfdataCollectors(), r.statusCollector()) {
		c.Collect(ch)
	}
}

func (r *Result) statusCollector() *prometheus.GaugeVec {
	status := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: prometheus.BuildFQName(r.NameSpace, r.Check, "status_code"),
		Help: fmt.Sprintf("Nagios compatible status code for %s", r.Check),
	}, []string{"item", "status"})

	sname := strings.ReplaceAll(r.Name, `"`, `.`)
	status.WithLabelValues(sname, string(r.Status)).Set(float64(r.nagiosCode()))

	return status
}

func (r *Result) perfdataCollectors() []*prometheus.GaugeVec {
	var res []*prometheus.GaugeVec

	sname := strings.ReplaceAll(r.Name, `"`, `.`)
	for _, pd := range r.PerfData {
		help := fmt.Sprintf("Data about the NATS CLI check %s", r.Check)
		if pd.Help != "" {
			help = pd.Help
		}

		gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: prometheus.BuildFQName(r.NameSpace, r.Check, pd.Name),
			Help: help,
		}, []string{"item"})
		gauge.WithLabelValues(sname).Set(pd.Value)
		res = append(res, gauge)
	}

	return res
}

func (r *Result) renderPrometheus() string {
	if r.Check == "" {
		r.Check = r.Name
	}

	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	for _, gauge := range r.perfdataCollectors() {
		prometheus.MustRegister(gauge)
	}

	status := r.statusCollector()
	prometheus.MustRegister(status)

	var buf bytes.Buffer

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		panic(err)
	}

	for _, mf := range mfs {
		_, err = expfmt.MetricFamilyToText(&buf, mf)
		if err != nil {
			panic(err)
		}
	}

	return buf.String()
}

func (r *Result) renderJSON() string {
	res, _ := json.MarshalIndent(r, "", "  ")
	return string(res)
}

func (r *Result) renderNagios() string {
	res := []string{r.Name}
	for _, c := range r.Criticals {
		res = append(res, fmt.Sprintf("Crit:%s", c))
	}

	for _, w := range r.Warnings {
		res = append(res, fmt.Sprintf("Warn:%s", w))
	}

	if r.Output != "" {
		res = append(res, r.Output)
	} else if len(r.OKs) > 0 {
		for _, ok := range r.OKs {
			res = append(res, fmt.Sprintf("OK:%s", ok))
		}
	}

	if len(r.PerfData) == 0 {
		return fmt.Sprintf("%s %s", r.Status, strings.Join(res, " "))
	}

	return fmt.Sprintf("%s %s | %s", r.Status, strings.Join(res, " "), r.PerfData)
}

func (r *Result) prepare() {
	if r.Status == "" {
		r.Status = UnknownStatus
	}
	if r.PerfData == nil {
		r.PerfData = PerfData{}
	}

	switch {
	case len(r.Criticals) > 0:
		r.Status = CriticalStatus
	case len(r.Warnings) > 0:
		r.Status = WarningStatus
	default:
		r.Status = OKStatus
	}
}

func (r *Result) String() string {
	r.prepare()

	switch r.RenderFormat {
	case JSONFormat:
		return r.renderJSON()
	case PrometheusFormat:
		return r.renderPrometheus()
	case TextFormat:
		return r.renderHuman()
	default:
		return r.renderNagios()
	}
}

func (r *Result) GenericExit() {
	// GenericExit is often called in defer and this will swallow panics as it will exit before the panic handler is called
	// so we try to add some flavor here at least
	err := recover()
	if err != nil {
		r.Critical("check caused a panic: %v", err)
		if r.Trace {
			debug.PrintStack()
		}
	}

	if r.OutFile != "" {
		f, err := os.CreateTemp(filepath.Dir(r.OutFile), "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file failed: %s", err)
			os.Exit(1)
		}
		defer os.Remove(f.Name())

		_, err = fmt.Fprintln(f, r.String())
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file write failed: %s", err)
			os.Exit(1)
		}

		err = f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file write failed: %s", err)
			os.Exit(1)
		}

		err = os.Chmod(f.Name(), 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file mode change failed: %s", err)
			os.Exit(1)
		}

		err = os.Rename(f.Name(), r.OutFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file rename failed: %s", err)
		}

		os.Exit(1)
	}

	fmt.Println(r.String())

	r.Exit()
}

func f(v any) string {
	switch x := v.(type) {
	case []string:
		return strings.Join(x, ", ")
	case time.Duration:
		return humanizeDuration(x)
	case time.Time:
		return x.Local().Format("2006-01-02 15:04:05")
	case bool:
		return fmt.Sprintf("%t", x)
	case uint:
		return humanize.Comma(int64(x))
	case uint32:
		return humanize.Comma(int64(x))
	case uint16:
		return humanize.Comma(int64(x))
	case uint64:
		return humanize.Comma(int64(x))
	case int:
		return humanize.Comma(int64(x))
	case int32:
		return humanize.Comma(int64(x))
	case int64:
		return humanize.Comma(x)
	case float32:
		return humanize.CommafWithDigits(float64(x), 3)
	case float64:
		return humanize.CommafWithDigits(x, 3)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func humanizeDuration(d time.Duration) string {
	if d < time.Millisecond {
		return d.Round(time.Microsecond).String()
	}

	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}

	if d == math.MaxInt64 {
		return "never"
	}

	tsecs := d / time.Second
	tmins := tsecs / 60
	thrs := tmins / 60
	tdays := thrs / 24
	tyrs := tdays / 365

	if tyrs > 0 {
		return fmt.Sprintf("%dy%dd%dh%dm%ds", tyrs, tdays%365, thrs%24, tmins%60, tsecs%60)
	}

	if tdays > 0 {
		return fmt.Sprintf("%dd%dh%dm%ds", tdays, thrs%24, tmins%60, tsecs%60)
	}

	if thrs > 0 {
		return fmt.Sprintf("%dh%dm%ds", thrs, tmins%60, tsecs%60)
	}

	if tmins > 0 {
		return fmt.Sprintf("%dm%ds", tmins, tsecs%60)
	}

	return fmt.Sprintf("%.2fs", d.Seconds())
}
