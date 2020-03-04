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

package jsm_test

import (
	"testing"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
)

func TestParseJSMsgMetadata(t *testing.T) {
	i, err := jsm.ParseJSMsgMetadata(&nats.Msg{Reply: "$JS.ACK.ORDERS.NEW.1.2.3"})
	checkErr(t, err, "msg parse failed")

	if i.Stream() != "ORDERS" {
		t.Fatalf("expected ORDERS got %s", i.Stream())
	}

	if i.Consumer() != "NEW" {
		t.Fatalf("expected NEW got %s", i.Consumer())
	}

	if i.Delivered() != 1 {
		t.Fatalf("expceted 1 got %d", i.Delivered())
	}

	if i.StreamSequence() != 2 {
		t.Fatalf("expceted 2 got %d", i.StreamSequence())
	}

	if i.ConsumerSequence() != 3 {
		t.Fatalf("expceted 3 got %d", i.ConsumerSequence())
	}
}
