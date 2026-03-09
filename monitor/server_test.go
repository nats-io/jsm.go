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
	"fmt"
	"strings"
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
		err := monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "x", Resolver: vzResolver})
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
		err := monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "example.net", Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)
		assertListEquals(t, check.Criticals, "result from wrong server \"other\"")
	})

	t.Run("wrong server stops subsequent checks", func(t *testing.T) {
		// JetStreamRequired is true, but the resolver returns a wrong server name.
		// After detecting the wrong server the function must return without running
		// any further checks, so OKs must remain empty.
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return &server.Varz{Name: "impostor", JetStream: server.JetStreamVarz{Config: &server.JetStreamConfig{}}}, nil
		}

		check := &monitor.Result{}
		err := monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{
			Name:              "example.net",
			JetStreamRequired: true,
			Resolver:          vzResolver,
		})
		checkErr(t, err, "check failed: %v", err)
		assertListEquals(t, check.Criticals, "result from wrong server \"impostor\"")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Warnings)
	})

	t.Run("resolver error", func(t *testing.T) {
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return nil, fmt.Errorf("timed out")
		}

		check := &monitor.Result{}
		err := monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "example.net", Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)
		if len(check.Criticals) != 1 || !strings.HasPrefix(check.Criticals[0], "varz failed:") {
			t.Fatalf("expected varz failed critical, got: %v", check.Criticals)
		}
	})

	t.Run("jetstream", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net"}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		check := &monitor.Result{}
		err := monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "example.net", JetStreamRequired: true, Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListEquals(t, check.Criticals, "JetStream not enabled")

		vz.JetStream.Config = &server.JetStreamConfig{}
		check = &monitor.Result{}
		err = monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "example.net", JetStreamRequired: true, Resolver: vzResolver})
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
		err := monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "example.net", TLSRequired: true, Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListEquals(t, check.Criticals, "TLS not required")

		vz.TLSRequired = true
		check = &monitor.Result{}
		err = monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "example.net", TLSRequired: true, Resolver: vzResolver})
		checkErr(t, err, "check failed: %v", err)
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.OKs, "TLS required")
	})

	t.Run("tls cert expiry", func(t *testing.T) {
		now := time.Now().UTC()
		vz := &server.Varz{Name: "example.net", Now: now}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		opts := monitor.CheckServerOptions{
			Name:              "example.net",
			TLSExpireWarning:  "30d",
			TLSExpireCritical: "7d",
			Resolver:          vzResolver,
		}

		vz.TLSCertNotAfter = now.Add(-1 * time.Hour)
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		if len(check.Criticals) != 1 || !strings.HasPrefix(check.Criticals[0], "server TLS certificate expired") {
			t.Fatalf("expected expired cert critical, got: %v", check.Criticals)
		}
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)

		vz.TLSCertNotAfter = now.Add(24 * time.Hour)
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		if len(check.Criticals) != 1 || !strings.HasPrefix(check.Criticals[0], "server TLS certificate expires in") {
			t.Fatalf("expected expiring cert critical, got: %v", check.Criticals)
		}
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)

		vz.TLSCertNotAfter = now.Add(10 * 24 * time.Hour)
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		if len(check.Warnings) != 1 || !strings.HasPrefix(check.Warnings[0], "server TLS certificate expires in") {
			t.Fatalf("expected expiring cert warning, got: %v", check.Warnings)
		}
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.OKs)

		vz.TLSCertNotAfter = now.Add(45 * 24 * time.Hour)
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		if len(check.OKs) != 1 || !strings.HasPrefix(check.OKs[0], "server TLS certificate expires in") {
			t.Fatalf("expected expiring cert ok, got: %v", check.OKs)
		}
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)

		opts.TLSExpireWarning = ""
		opts.TLSExpireCritical = ""
		vz.TLSCertNotAfter = now.Add(-1 * time.Hour)
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)

		opts.TLSExpireWarning = "30d"
		opts.TLSExpireCritical = "7d"
		vz.TLSCertNotAfter = time.Time{}
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "TLS certificate expiry thresholds configured but no TLS certificates found")
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)

		opts.TLSExpireWarning = "30d"
		opts.TLSExpireCritical = ""
		vz.TLSCertNotAfter = now.Add(24 * time.Hour)
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "TLS certificate expiry requires both warning and critical thresholds")
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)

		opts.TLSExpireWarning = ""
		opts.TLSExpireCritical = "7d"
		vz.TLSCertNotAfter = now.Add(24 * time.Hour)
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "TLS certificate expiry requires both warning and critical thresholds")
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)
	})

	t.Run("authentication", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net"}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		check := &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "example.net", AuthenticationRequired: true, Resolver: vzResolver}))
		assertListEquals(t, check.Criticals, "Authentication not required")

		vz.AuthRequired = true
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, monitor.CheckServerOptions{Name: "example.net", AuthenticationRequired: true, Resolver: vzResolver}))
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.OKs, "Authentication required")
	})

	t.Run("uptime", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net"}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		opts := monitor.CheckServerOptions{
			Name:     "example.net",
			Resolver: vzResolver,
		}

		// invalid thresholds
		check := &monitor.Result{}
		opts.UptimeCritical = 20 * time.Minute.Seconds()
		opts.UptimeWarning = 10 * time.Minute.Seconds()

		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Up invalid thresholds")
		assertListIsEmpty(t, check.OKs)

		// critical uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1 * time.Second)
		opts.UptimeCritical = 10 * time.Minute.Seconds()
		opts.UptimeWarning = 20 * time.Minute.Seconds()
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Up 1.00s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=1.0000s;1200.0000;600.000")

		// critical uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-599 * time.Second)
		opts.UptimeCritical = 10 * time.Minute.Seconds()
		opts.UptimeWarning = 20 * time.Minute.Seconds()
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Up 9m59s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=599.0000s;1200.0000;600.000")

		// critical uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-600 * time.Second)
		opts.UptimeCritical = 10 * time.Minute.Seconds()
		opts.UptimeWarning = 20 * time.Minute.Seconds()
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Up 10m0s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=600.0000s;1200.0000;600.000")

		// critical -> warning uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-601 * time.Second)
		opts.UptimeCritical = 10 * time.Minute.Seconds()
		opts.UptimeWarning = 20 * time.Minute.Seconds()
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.Warnings, "Up 10m1s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=601.0000s;1200.0000;600.000")

		// warning uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1199 * time.Second)
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.Warnings, "Up 19m59s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=1199.0000s;1200.0000;600.000")

		// warning uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1200 * time.Second)
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.Warnings, "Up 20m0s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "uptime=1200.0000s;1200.0000;600.000")

		// ok uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1201 * time.Second)
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertListEquals(t, check.OKs, "Up 20m1s")
		assertHasPDItem(t, check, "uptime=1201.0000s;1200.0000;600.0000")

		// ok uptime
		check = &monitor.Result{}
		vz.Now = time.Now()
		vz.Start = vz.Now.Add(-1260 * time.Second)
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
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

		opts := monitor.CheckServerOptions{
			Name:     "example.net",
			Resolver: vzResolver,
		}

		// invalid thresholds
		opts.CPUCritical = 60
		opts.CPUWarning = 70
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "CPU invalid thresholds")
		assertListIsEmpty(t, check.OKs)

		// critical cpu
		opts.CPUCritical = 50
		opts.CPUWarning = 30
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "CPU 50.00")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "cpu=50%;30;50")

		// warning cpu
		opts.CPUCritical = 60
		opts.CPUWarning = 50
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Warnings, "CPU 50.00")
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "cpu=50%;50;60")

		// ok cpu
		opts.CPUCritical = 80
		opts.CPUWarning = 70
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
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

		opts := monitor.CheckServerOptions{
			Name:     "example.net",
			Resolver: vzResolver,
		}

		// critical connections
		opts.ConnectionsCritical = 1024
		opts.ConnectionsWarning = 800
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Connections 1024.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "connections=1024;800;1024")

		// critical connections reverse
		opts.ConnectionsCritical = 1200
		opts.ConnectionsWarning = 1300
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Connections 1024.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "connections=1024;1300;1200")

		// warn connections
		opts.ConnectionsCritical = 2000
		opts.ConnectionsWarning = 1024
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Warnings, "Connections 1024.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "connections=1024;1024;2000")

		// warn connections reverse
		opts.ConnectionsCritical = 1000
		opts.ConnectionsWarning = 1300
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Warnings, "Connections 1024.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "connections=1024;1300;1000")

		// ok connections
		opts.ConnectionsCritical = 2000
		opts.ConnectionsWarning = 1300
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.OKs, "Connections 1024.00")
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "connections=1024;1300;2000")

		// ok connections reverse
		opts.ConnectionsCritical = 800
		opts.ConnectionsWarning = 900
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.Criticals)
		assertListEquals(t, check.OKs, "Connections 1024.00")
		assertHasPDItem(t, check, "connections=1024;900;800")
	})

	t.Run("subscriptions", func(t *testing.T) {
		vz := &server.Varz{Name: "example.net", Subscriptions: 512}
		vzResolver := func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) {
			return vz, nil
		}

		opts := monitor.CheckServerOptions{
			Name:     "example.net",
			Resolver: vzResolver,
		}

		// critical subscriptions
		opts.SubscriptionsCritical = 512
		opts.SubscriptionsWarning = 400
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Subscriptions 512.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "subscriptions=512;400;512")

		// critical subscriptions reverse (below minimum)
		opts.SubscriptionsCritical = 600
		opts.SubscriptionsWarning = 700
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Criticals, "Subscriptions 512.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "subscriptions=512;700;600")

		// warning subscriptions
		opts.SubscriptionsCritical = 1000
		opts.SubscriptionsWarning = 512
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Warnings, "Subscriptions 512.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "subscriptions=512;512;1000")

		// warning subscriptions reverse
		opts.SubscriptionsCritical = 500
		opts.SubscriptionsWarning = 600
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.Warnings, "Subscriptions 512.00")
		assertListIsEmpty(t, check.OKs)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "subscriptions=512;600;500")

		// ok subscriptions
		opts.SubscriptionsCritical = 1000
		opts.SubscriptionsWarning = 700
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.OKs, "Subscriptions 512.00")
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "subscriptions=512;700;1000")

		// ok subscriptions reverse (above minimum)
		opts.SubscriptionsCritical = 400
		opts.SubscriptionsWarning = 450
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
		assertListEquals(t, check.OKs, "Subscriptions 512.00")
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "subscriptions=512;450;400")
	})

	t.Run("tls cert expiry per component", func(t *testing.T) {
		now := time.Now().UTC()
		opts := monitor.CheckServerOptions{
			Name:              "example.net",
			TLSExpireWarning:  "30d",
			TLSExpireCritical: "7d",
		}

		expiredAt := now.Add(-1 * time.Hour)
		soonAt := now.Add(24 * time.Hour)     // within critical (7d)
		warnAt := now.Add(10 * 24 * time.Hour) // within warning (30d)
		okAt := now.Add(45 * 24 * time.Hour)   // beyond warning

		for _, tc := range []struct {
			name    string
			setVarz func(vz *server.Varz, t time.Time)
			prefix  string
		}{
			{
				name:   "cluster",
				setVarz: func(vz *server.Varz, t time.Time) { vz.Cluster.TLSCertNotAfter = t },
				prefix: "cluster TLS certificate",
			},
			{
				name:   "gateway",
				setVarz: func(vz *server.Varz, t time.Time) { vz.Gateway.TLSCertNotAfter = t },
				prefix: "gateway TLS certificate",
			},
			{
				name:   "leafnode",
				setVarz: func(vz *server.Varz, t time.Time) { vz.LeafNode.TLSCertNotAfter = t },
				prefix: "leafnode TLS certificate",
			},
			{
				name:   "mqtt",
				setVarz: func(vz *server.Varz, t time.Time) { vz.MQTT.TLSCertNotAfter = t },
				prefix: "mqtt TLS certificate",
			},
			{
				name:   "websocket",
				setVarz: func(vz *server.Varz, t time.Time) { vz.Websocket.TLSCertNotAfter = t },
				prefix: "websocket TLS certificate",
			},
		} {
			t.Run(tc.name+" expired", func(t *testing.T) {
				vz := &server.Varz{Name: "example.net", Now: now}
				tc.setVarz(vz, expiredAt)
				opts.Resolver = func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) { return vz, nil }
				check := &monitor.Result{}
				assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
				if len(check.Criticals) != 1 || !strings.HasPrefix(check.Criticals[0], tc.prefix+" expired") {
					t.Fatalf("expected %s expired critical, got: %v", tc.name, check.Criticals)
				}
			})

			t.Run(tc.name+" critical", func(t *testing.T) {
				vz := &server.Varz{Name: "example.net", Now: now}
				tc.setVarz(vz, soonAt)
				opts.Resolver = func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) { return vz, nil }
				check := &monitor.Result{}
				assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
				if len(check.Criticals) != 1 || !strings.HasPrefix(check.Criticals[0], tc.prefix+" expires in") {
					t.Fatalf("expected %s expiring critical, got: %v", tc.name, check.Criticals)
				}
			})

			t.Run(tc.name+" warning", func(t *testing.T) {
				vz := &server.Varz{Name: "example.net", Now: now}
				tc.setVarz(vz, warnAt)
				opts.Resolver = func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) { return vz, nil }
				check := &monitor.Result{}
				assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
				if len(check.Warnings) != 1 || !strings.HasPrefix(check.Warnings[0], tc.prefix+" expires in") {
					t.Fatalf("expected %s expiring warning, got: %v", tc.name, check.Warnings)
				}
			})

			t.Run(tc.name+" ok", func(t *testing.T) {
				vz := &server.Varz{Name: "example.net", Now: now}
				tc.setVarz(vz, okAt)
				opts.Resolver = func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) { return vz, nil }
				check := &monitor.Result{}
				assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
				if len(check.OKs) != 1 || !strings.HasPrefix(check.OKs[0], tc.prefix+" expires in") {
					t.Fatalf("expected %s expiring ok, got: %v", tc.name, check.OKs)
				}
			})
		}

		// hasTLSCertNotAfter: only a component cert present, no server cert
		t.Run("component cert triggers hasTLSCertNotAfter", func(t *testing.T) {
			vz := &server.Varz{Name: "example.net", Now: now}
			vz.Cluster.TLSCertNotAfter = expiredAt
			opts.Resolver = func(_ *nats.Conn, _ string, _ time.Duration) (*server.Varz, error) { return vz, nil }
			check := &monitor.Result{}
			assertNoError(t, monitor.CheckServer("", nil, check, time.Second, opts))
			// Should not see "no TLS certificates found" — it found the cluster cert
			for _, c := range check.Criticals {
				if strings.Contains(c, "no TLS certificates found") {
					t.Fatalf("should have found cluster cert but got: %v", check.Criticals)
				}
			}
		})
	})
}
