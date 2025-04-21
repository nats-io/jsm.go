// Copyright 2020 The NATS Authors
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

package test

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nats-io/jsm.go"
	testapi "github.com/nats-io/jsm.go/test/testing_client/api"
	"github.com/nats-io/jsm.go/test/testing_client/srvtest"
)

func TestStream_Snapshot(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		nc := mgr.NatsConn()

		stream, err := mgr.NewStream("m1", jsm.MemoryStorage(), jsm.Subjects("memtest"))
		checkErr(t, err, "create failed")

		_, err = stream.SnapshotToDirectory(context.Background(), "/tmp")
		if !errors.Is(err, jsm.ErrMemoryStreamNotSupported) {
			t.Fatalf("expected memory error, got %v", err)
		}

		stream, err = mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		_, err = stream.NewConsumer(jsm.DurableName("c"))
		checkErr(t, err, "consumer failed")

		for i := 0; i <= 3000; i++ {
			nc.Publish(stream.Subjects()[0], []byte(RandomString(5480)))
		}

		preState, err := stream.State()
		checkErr(t, err, "state retrieve failed")

		td, err := os.MkdirTemp("", "")
		checkErr(t, err, "temp dir failed")
		defer os.RemoveAll(td)

		_, err = stream.SnapshotToDirectory(context.Background(), td, jsm.SnapshotConsumers(), jsm.SnapshotHealthCheck())
		checkErr(t, err, "snapshot failed")

		checkErr(t, stream.Delete(), "delete failed")

		// restore same
		_, postRestoreState, err := mgr.RestoreSnapshotFromDirectory(context.Background(), "q1", td)
		checkErr(t, err, "restore failed")
		if postRestoreState == nil {
			t.Fatalf("got a nil post restore state")
		}

		if !reflect.DeepEqual(preState, *postRestoreState) {
			t.Fatalf("pre state does not match post restore state")
		}

		stream, err = mgr.LoadStream("q1")
		checkErr(t, err, "load failed")

		postState, err := stream.State()
		checkErr(t, err, "state failed")

		if !reflect.DeepEqual(preState, postState) {
			t.Fatalf("pre state does not match post state")
		}
		checkErr(t, stream.Delete(), "delete failed")

		// restore with new config
		cfg := stream.Configuration()
		cfg.Subjects = []string{"js.in.q3"}

		_, _, err = mgr.RestoreSnapshotFromDirectory(context.Background(), "q3", td, jsm.RestoreConfiguration(cfg))
		if err.Error() != "stream name may not be changed during restore" {
			t.Fatalf("expected rename error")
		}

		_, postRestoreState, err = mgr.RestoreSnapshotFromDirectory(context.Background(), "q1", td, jsm.RestoreConfiguration(cfg))
		checkErr(t, err, "restore failed")
		if postRestoreState == nil {
			t.Fatalf("got a nil post restore state")
		}
		if !reflect.DeepEqual(preState, *postRestoreState) {
			t.Fatalf("pre state does not match post restore state")
		}

		stream, err = mgr.LoadStream("q1")
		checkErr(t, err, "load failed")

		postState, err = stream.State()
		checkErr(t, err, "state failed")
		if !reflect.DeepEqual(preState, postState) {
			t.Fatalf("pre state does not match post state")
		}
		if !cmp.Equal(stream.Subjects(), cfg.Subjects) {
			t.Fatalf("stream config replace did not work")
		}
	})
}

func RandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
