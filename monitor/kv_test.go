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

package monitor

import (
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func checkErr(t *testing.T, err error, format string, a ...any) {
	t.Helper()
	if err == nil {
		return
	}

	t.Fatalf(format, a...)
}

func assertHasPDItem(t *testing.T, check *Result, items ...string) {
	t.Helper()

	if len(items) == 0 {
		t.Fatalf("no items to assert")
	}

	pd := check.PerfData.String()
	for _, i := range items {
		if !strings.Contains(pd, i) {
			t.Fatalf("did not contain item: '%s': %s", i, pd)
		}
	}
}

func assertListIsEmpty(t *testing.T, list []string) {
	t.Helper()

	if len(list) > 0 {
		t.Fatalf("invalid items: %v", list)
	}
}

func assertListEquals(t *testing.T, list []string, vals ...string) {
	t.Helper()

	sort.Strings(list)
	sort.Strings(vals)

	if !cmp.Equal(list, vals) {
		t.Fatalf("invalid items: %v", list)
	}
}

func withJetStream(t *testing.T, cb func(srv *server.Server, nc *nats.Conn)) {
	t.Helper()

	dir, err := os.MkdirTemp("", "")
	checkErr(t, err, "could not create temporary js store: %v", err)
	defer os.RemoveAll(dir)

	srv, err := server.NewServer(&server.Options{
		Port:      -1,
		StoreDir:  dir,
		JetStream: true,
	})
	checkErr(t, err, "could not start js server: %v", err)

	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		t.Errorf("nats server did not start")
	}
	defer func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	}()

	nc, err := nats.Connect(srv.ClientURL())
	checkErr(t, err, "could not connect client to server @ %s: %v", srv.ClientURL(), err)
	defer nc.Close()

	cb(srv, nc)
}

func TestCheckKVBucketAndKey(t *testing.T) {
	t.Run("Bucket", func(t *testing.T) {
		withJetStream(t, func(_ *server.Server, nc *nats.Conn) {
			check := &Result{}
			err := CheckKVBucketAndKey(nc, check, KVCheckOptions{
				Bucket: "TEST",
			})
			checkErr(t, err, "check failed: %v", err)
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.OKs)
			assertListEquals(t, check.Criticals, "bucket TEST not found")

			js, err := nc.JetStream()
			checkErr(t, err, "js context failed")

			_, err = js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "TEST"})
			checkErr(t, err, "kv create failed")

			check = &Result{}
			err = CheckKVBucketAndKey(nc, check, KVCheckOptions{
				Bucket:         "TEST",
				ValuesCritical: -1,
				ValuesWarning:  -1,
			})
			checkErr(t, err, "check failed: %v", err)
			assertHasPDItem(t, check, "values=0 bytes=0B replicas=1")
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.OKs, "bucket TEST")
		})
	})

	t.Run("Values", func(t *testing.T) {
		withJetStream(t, func(srv *server.Server, nc *nats.Conn) {
			js, err := nc.JetStream()
			checkErr(t, err, "js context failed")

			bucket, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "TEST"})
			checkErr(t, err, "kv create failed: %v", err)

			opts := KVCheckOptions{
				Bucket:         "TEST",
				ValuesWarning:  1,
				ValuesCritical: 2,
			}

			check := &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.OKs, "0 values", "bucket TEST")

			_, err = bucket.PutString("K", "V")
			checkErr(t, err, "pub failed")

			check = &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertHasPDItem(t, check, "values=1;1;2 bytes=41B replicas=1")
			assertListEquals(t, check.OKs, "bucket TEST")
			assertListEquals(t, check.Warnings, "1 values")
			assertListIsEmpty(t, check.Criticals)

			_, err = bucket.PutString("K1", "V")
			checkErr(t, err, "pub failed")

			check = &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertHasPDItem(t, check, "values=2;1;2 bytes=83B replicas=1")
			assertListEquals(t, check.OKs, "bucket TEST")
			assertListEquals(t, check.Criticals, "2 values")
			assertListIsEmpty(t, check.Warnings)

			// now test inverse logic

			opts.ValuesCritical = 3
			opts.ValuesWarning = 5

			_, err = bucket.PutString("K2", "V")
			checkErr(t, err, "pub failed")

			check = &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertHasPDItem(t, check, "values=3;5;3 bytes=125B replicas=1")
			assertListIsEmpty(t, check.Warnings)
			assertListEquals(t, check.OKs, "bucket TEST")
			assertListEquals(t, check.Criticals, "3 values")

			_, err = bucket.PutString("K3", "V")
			checkErr(t, err, "pub failed")

			check = &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertHasPDItem(t, check, "values=4;5;3 bytes=167B replicas=1")
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.OKs, "bucket TEST")
			assertListEquals(t, check.Warnings, "4 values")

			_, err = bucket.PutString("K4", "V")
			checkErr(t, err, "pub failed")
			_, err = bucket.PutString("K5", "V")
			checkErr(t, err, "pub failed")

			check = &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertHasPDItem(t, check, "values=6;5;3 bytes=251B replicas=1")
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.OKs, "bucket TEST", "6 values")
		})
	})

	t.Run("Key", func(t *testing.T) {
		withJetStream(t, func(srv *server.Server, nc *nats.Conn) {
			js, err := nc.JetStream()
			checkErr(t, err, "js context failed")

			bucket, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "TEST"})
			checkErr(t, err, "kv create failed")

			opts := KVCheckOptions{
				Bucket:         "TEST",
				Key:            "KEY",
				ValuesWarning:  -1,
				ValuesCritical: -1,
			}

			check := &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertListIsEmpty(t, check.Warnings)
			assertListEquals(t, check.OKs, "bucket TEST")
			assertListEquals(t, check.Criticals, "key KEY not found")

			_, err = bucket.Put("KEY", []byte("VAL"))
			checkErr(t, err, "put failed")

			check = &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertListIsEmpty(t, check.Warnings)
			assertListEquals(t, check.OKs, "bucket TEST", "key KEY found")
			assertListIsEmpty(t, check.Criticals)

			bucket.Delete("KEY")
			check = &Result{}
			err = CheckKVBucketAndKey(nc, check, opts)
			checkErr(t, err, "check failed: %v", err)
			assertListIsEmpty(t, check.Warnings)
			assertListEquals(t, check.OKs, "bucket TEST")
			assertListEquals(t, check.Criticals, "key KEY not found")
		})
	})
}
