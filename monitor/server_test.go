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

package monitor_test

import (
	"testing"
	"time"

	"github.com/nats-io/jsm.go/monitor"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestCheckVarz(t *testing.T) {
	t.Run("nil data", func(t *testing.T) {
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return nil, nil
		}

		check := &monitor.Result{}
		err := monitor.CheckServer(nil, check, time.Second, monitor.ServerCheckOptions{Name: "x", Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)
		assertListEquals(t, check.Criticals, "no data received")
	})

	t.Run("wrong server", func(t *testing.T) {
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return &server.Varz{Name: "other"}, nil
		}

		check := &monitor.Result{}
		err := monitor.CheckServer(nil, check, time.Second, monitor.ServerCheckOptions{Name: "example.net", Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)
		assertListEquals(t, check.Criticals, "result from wrong server \"other\"")
	})

	t.Run("jetstream", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net"}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		check := &monitor.Result{}
		err := monitor.CheckServer(nil, check, time.Second, monitor.ServerCheckOptions{Name: "example.net", JetStreamRequired: true, Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListEquals(t, check.Criticals, "JetStream not enabled")

		vz.JetStream.Config = &server.JetStreamConfig{}
		check = &monitor.Result{}
		err = monitor.CheckServer(nil, check, time.Second, monitor.ServerCheckOptions{Name: "example.net", JetStreamRequired: true, Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.OKs, "JetStream enabled")
	})

	t.Run("tls", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net"}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		check := &monitor.Result{}
		err := monitor.CheckServer(nil, check, time.Second, monitor.ServerCheckOptions{Name: "example.net", TLSRequired: true, Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListEquals(t, check.Criticals, "TLS not required")

		vz.TLSRequired = true
		check = &monitor.Result{}
		err = monitor.CheckServer(nil, check, time.Second, monitor.ServerCheckOptions{Name: "example.net", TLSRequired: true, Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.OKs, "TLS required")
	})

	t.Run("authentication", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net"}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		check := &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, monitor.ServerCheckOptions{Name: "example.net", AuthenticationRequired: true, Resolver: vzResolver}))
		assertListEquals(t, check.Criticals, "Authentication not required")

		vz.AuthRequired = true
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, monitor.ServerCheckOptions{Name: "example.net", AuthenticationRequired: true, Resolver: vzResolver}))
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.OKs, "Authentication required")
	})

	t.Run("uptime", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net"}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		opts := monitor.ServerCheckOptions{
			Name:     "example.net",
			Resolver: vzResolver,
		}

		// invalid thresholds
		check := &monitor.Result{}
		opts.UptimeCritical = 20 * time.Minute
		opts.UptimeWarning = 10 * time.Minute

		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Up invalid thresholds")
		assertListIsEmpty(t, check.OKs)

		// critical uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1 * time.Second)
		opts.UptimeCritical = 10 * time.Minute
		opts.UptimeWarning = 20 * time.Minute
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Up 1.00s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=1.0000s;1200.0000;600.000")

		// critical uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-599 * time.Second)
		opts.UptimeCritical = 10 * time.Minute
		opts.UptimeWarning = 20 * time.Minute
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Up 9m59s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=599.0000s;1200.0000;600.000")

		// critical uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-600 * time.Second)
		opts.UptimeCritical = 10 * time.Minute
		opts.UptimeWarning = 20 * time.Minute
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Up 10m0s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=600.0000s;1200.0000;600.000")

		// critical -> warning uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-601 * time.Second)
		opts.UptimeCritical = 10 * time.Minute
		opts.UptimeWarning = 20 * time.Minute
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.Warnings, "Up 10m1s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=601.0000s;1200.0000;600.000")

		// warning uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1199 * time.Second)
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.Warnings, "Up 19m59s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=1199.0000s;1200.0000;600.000")

		// warning uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1200 * time.Second)
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.Warnings, "Up 20m0s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=1200.0000s;1200.0000;600.000")

		// ok uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1201 * time.Second)
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertListEquals(t, check.OKs, "Up 20m1s")
		assertHasPDItem(t, check, "uptime=1201.0000s;1200.0000;600.0000")

		// ok uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1260 * time.Second)
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertListEquals(t, check.OKs, "Up 21m0s")
		assertHasPDItem(t, check, "uptime=1260.0000s;1200.0000;600.0000")
	})

	t.Run("cpu", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net", CPU: 50}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		opts := monitor.ServerCheckOptions{
			Name:     "example.net",
			Resolver: vzResolver,
		}

		// invalid thresholds
		opts.CPUCritical = 60
		opts.CPUWarning = 70
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "CPU invalid thresholds")
		assertListIsEmpty(t, check.OKs)

		// critical cpu
		opts.CPUCritical = 50
		opts.CPUWarning = 30
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "CPU 50.00")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "cpu=50%;30;50")

		// warning cpu
		opts.CPUCritical = 60
		opts.CPUWarning = 50
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Warnings, "CPU 50.00")
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "cpu=50%;50;60")

		// ok cpu
		opts.CPUCritical = 80
		opts.CPUWarning = 70
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.OKs, "CPU 50.00")
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "cpu=50%;70;80")
	})

	// memory not worth testing, its the shared logic with CPU

	t.Run("connections", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net", Connections: 1024}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		opts := monitor.ServerCheckOptions{
			Name:     "example.net",
			Resolver: vzResolver,
		}

		// critical connections
		opts.ConnectionsCritical = 1024
		opts.ConnectionsWarning = 800
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Connections 1024.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "connections=1024;800;1024")

		// critical connections reverse
		opts.ConnectionsCritical = 1200
		opts.ConnectionsWarning = 1300
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Connections 1024.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "connections=1024;1300;1200")

		// warn connections
		opts.ConnectionsCritical = 2000
		opts.ConnectionsWarning = 1024
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Warnings, "Connections 1024.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "connections=1024;1024;2000")

		// warn connections reverse
		opts.ConnectionsCritical = 1000
		opts.ConnectionsWarning = 1300
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.Warnings, "Connections 1024.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "connections=1024;1300;1000")

		// ok connections
		opts.ConnectionsCritical = 2000
		opts.ConnectionsWarning = 1300
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListEquals(t, check.OKs, "Connections 1024.00")
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "connections=1024;1300;2000")

		// ok connections reverse
		opts.ConnectionsCritical = 800
		opts.ConnectionsWarning = 900
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer(nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.OKs, "Connections 1024.00")
		assertHasPDItem(t, check, "connections=1024;900;800")
	})
}
