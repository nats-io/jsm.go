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

package test

import (
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

func TestApiLevelDetection(t *testing.T) {
	req := api.JSApiConsumerCreateRequest{
		Config: api.ConsumerConfig{
			PriorityGroups: []string{"a"},
			PriorityPolicy: api.PriorityPinnedClient,
		},
	}

	lvl, err := api.RequiredApiLevel(req)
	checkErr(t, err, "failed to determine level")
	if lvl != 1 {
		t.Fatalf("expected api level 1 but got %d", lvl)
	}

	req.Config.PriorityPolicy = api.PriorityPrioritized
	lvl, err = api.RequiredApiLevel(req)
	checkErr(t, err, "failed to determine level")
	if lvl != 2 {
		t.Fatalf("expected api level 2 but got %d", lvl)
	}

	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	s, err := mgr.NewStreamFromDefault("TEST", api.StreamConfig{}, jsm.Subjects("test.*"))
	checkErr(t, err, "create failed")

	sub, err := nc.SubscribeSync("$JS.API.CONSUMER.CREATE.>")
	checkErr(t, err, "create failed")

	_, err = s.NewConsumer(jsm.PrioritizedPriorityGroups("foo"))
	checkErr(t, err, "create failed")

	msg, err := sub.NextMsg(time.Second)
	checkErr(t, err, "next failed")

	v := msg.Header.Get(api.JSRequiredApiLevel)
	if v != "2" {
		t.Fatalf("expected api level 2 but got %s", v)
	}

	_, err = s.NewConsumer(jsm.PinnedClientPriorityGroups(time.Minute, "foo"))
	checkErr(t, err, "create failed")

	msg, err = sub.NextMsg(time.Second)
	checkErr(t, err, "next failed")

	v = msg.Header.Get(api.JSRequiredApiLevel)
	if v != "1" {
		t.Fatalf("expected api level 1 but got %s", v)
	}

	_, err = s.NewConsumer()
	checkErr(t, err, "create failed")

	msg, err = sub.NextMsg(time.Second)
	checkErr(t, err, "next failed")

	v = msg.Header.Get(api.JSRequiredApiLevel)
	if v != "" {
		t.Fatalf("expected no api level but got %s", v)
	}

}
