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

package tracing

import (
	"testing"
	"time"

	srv "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

// Do not change the order of these since some test relies on it.
var traces = []string{
	`{"server":{"name":"C2-S1","host":"0.0.0.0","id":"NCL2I64LB5UHUVK6D6IX5GXZ3QFCYDLNRHHOUZBI2TGUO32TK747XUBW","cluster":"C2","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":25,"time":"2024-01-30T16:57:20.287475Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["3"]},"msgsize":69},"hops":1,"events":[{"type":"in","kind":2,"cid":8,"name":"C1-S1","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":10,"name":"sub3","sub":"foo"},{"type":"eg","kind":4,"cid":6,"name":"C4-S1","hop":"3.1"}]}`,
	`{"server":{"name":"C1-S1","host":"0.0.0.0","id":"NBKYN6VUDABZLWVOWVFIUM7D63C7PWHS7TRWEQH2FR34ICEIAYBWJQS4","cluster":"C1","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":25,"time":"2024-01-30T16:57:20.287239Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"]},"msgsize":50},"hops":3,"events":[{"type":"in","kind":0,"cid":13,"name":"Tracer","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":14,"name":"sub1","sub":"foo"},{"type":"eg","kind":1,"cid":7,"name":"C1-S2","hop":"1"},{"type":"eg","kind":4,"cid":12,"name":"C3-S1","hop":"2"},{"type":"eg","kind":2,"cid":11,"name":"C2-S1","hop":"3"}]}`,
	`{"server":{"name":"C4-S1","host":"0.0.0.0","id":"NDPYNVX6FK76IWBTOKI32CGPW263E7PSCX7QCTUVMRMLAAUYMXVJL3SL","cluster":"C4","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":16,"time":"2024-01-30T16:57:20.287804Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["3.1"]},"msgsize":71},"events":[{"type":"in","kind":4,"cid":6,"name":"C2-S1","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":7,"name":"sub6","sub":"foo"}]}`,
	`{"server":{"name":"C3-S1","host":"0.0.0.0","id":"NCRW5HIUPRIZSNPUTCUTB2W4G5SRTHABL4VZK2QSMTARK6YKZMCKIAK3","cluster":"C3","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":16,"time":"2024-01-30T16:57:20.287804Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["2"]},"msgsize":69},"events":[{"type":"in","kind":4,"cid":6,"name":"C1-S1","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":11,"name":"sub4","sub":"foo"}]}`,
	`{"server":{"name":"C1-S2","host":"0.0.0.0","id":"NCPESRBATJ2BBJZOA2YNCC76VTHPH6ITORFR4H4CFULFCFIGMCIGRWOT","cluster":"C1","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":21,"time":"2024-01-30T16:57:20.287626Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["1"]},"msgsize":69},"hops":1,"events":[{"type":"in","kind":1,"cid":7,"name":"C1-S1","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":12,"name":"sub2","sub":"foo"},{"type":"eg","kind":4,"cid":11,"name":"C3-S2","hop":"1.1"}]}`,
	`{"server":{"name":"C3-S2","host":"0.0.0.0","id":"NAEV6N3HFB7UCMFZJV3SJCYOGZZQHMCEBOJFNDU5XU7RXW5SBAYWUHYA","cluster":"C3","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":21,"time":"2024-01-30T16:57:20.288014Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["1.1"]},"msgsize":71},"hops":2,"events":[{"type":"in","kind":4,"cid":8,"name":"C1-S2","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":13,"name":"sub5","sub":"foo"},{"type":"eg","kind":4,"cid":11,"name":"C5-S1","hop":"1.1.1"},{"type":"eg","kind":4,"cid":12,"name":"C5-S2","hop":"1.1.2"}]}`,
	`{"server":{"name":"C5-S2","host":"0.0.0.0","id":"NCHFFZLV24NDNEC4AQQB2JUZ2WVW7WWMAUSA6XMAIDAI75AOYGXTAJ2T","cluster":"C5","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":15,"time":"2024-01-30T16:57:20.28862Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["1.1.2"]},"msgsize":73},"events":[{"type":"in","kind":4,"cid":6,"name":"C3-S2","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":9,"name":"sub8","sub":"foo"}]}`,
	`{"server":{"name":"C5-S1","host":"0.0.0.0","id":"NB27RT2XQZJS2HLWOONIMZOI6ADDZWZENGKDTP25NAR4C62IEHSYNYNW","cluster":"C5","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":15,"time":"2024-01-30T16:57:20.289857Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["1.1.1"]},"msgsize":73},"events":[{"type":"in","kind":4,"cid":6,"name":"C3-S2","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":9,"name":"sub7","sub":"foo"}]}`,
}

// The traces above represent the following network toplogy
//
//
// ===================                       ===================
// =   C1 cluster    =                       =   C2 cluster    =
// ===================   <--- Gateway --->   ===================
// = C1-S1 <-> C1-S2 =                       =      C2-S1      =
// ===================                       ===================
//    ^          ^                                    ^
//    | Leafnode |                                    | Leafnode
//    |          |                                    |
// ===================                       ===================
// =    C3 cluster   =                       =    C4 cluster   =
// ===================                       ===================
// = C3-S1 <-> C3-S2 =                       =       C4-S1     =
// ===================                       ===================
//                ^
//                | Leafnode
//            |-------|
//       ===================
//       =    C5 cluster   =
//       ===================
//       = C5-S1 <-> C5-S2 =
//       ===================
//

func setupTest(t *testing.T) (*srv.Server, *nats.Conn, *nats.Subscription) {
	t.Helper()
	// We run a simple server and will mock trace returned as if they were
	// coming from a complex super-cluster with leafnodes setup.
	s := test.RunDefaultServer()

	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		s.Shutdown()
		t.Fatalf("Error on connect: %v", err)
	}

	// Create a subscription on "trace.*" and we will get trace on "trace.id"
	sub, err := nc.SubscribeSync("trace.*")
	if err != nil {
		nc.Close()
		s.Shutdown()
		t.Fatalf("Error on subscribe: %v", err)
	}
	return s, nc, sub
}

func TestMsgTracingBasic(t *testing.T) {
	s, nc, sub := setupTest(t)
	defer s.Shutdown()
	defer nc.Close()

	// Produce the traces. The `traces` slice has them already in some random order.
	for _, tr := range traces {
		nc.Publish("trace.id", []byte(tr))
	}
	e, err := GetMsgTrace(sub, "trace.id", time.Second, nil)
	if err != nil {
		t.Fatalf("Error getting trace: %v", err)
	}
	// We are not testing the unmarshal'ing per-se here, this is done in the server
	// and there are tests for that. We are just making sure that we have linked
	// the trace events as expected.

	// The event we get should be from the origin server and be "C1-S1".
	if sn := e.Server.Name; sn != "C1-S1" {
		t.Fatalf("Expected server C1-S1, got %q", sn)
	}

	checkEgressAndLink := func(eg *srv.MsgTraceEgress, sname string) {
		t.Helper()
		if sn := eg.Name; sn != sname {
			t.Fatalf("Expected server %q, got %q", sname, sn)
		}
		if eg.Link == nil {
			t.Fatal("Link should be set, but it was not")
		}
		if sn := eg.Link.Server.Name; sn != sname {
			t.Fatalf("Expected linked server to be %q, got %q", sname, sn)
		}
	}

	// The egress at pos 2 is for the ROUTER C1-S2, and the trace for that should be
	// in the Link field.
	c1s1Eg := e.Egresses()
	checkEgressAndLink(c1s1Eg[1], "C1-S2")
	// The next should be the LEAF C3-S1
	checkEgressAndLink(c1s1Eg[2], "C3-S1")
	// Finally, the last is the GATEWAY C2-S1
	checkEgressAndLink(c1s1Eg[3], "C2-S1")

	// Now use the server C1-S2 and check its links
	c1s2 := c1s1Eg[1].Link
	c1s2Eg := c1s2.Egresses()
	checkEgressAndLink(c1s2Eg[1], "C3-S2")

	// Check C2-S1
	c2s1 := c1s1Eg[3].Link
	c2s1Eg := c2s1.Egresses()
	checkEgressAndLink(c2s1Eg[1], "C4-S1")

	// Check C3-S2
	c3s2 := c1s2Eg[1].Link
	c3s2Eg := c3s2.Egresses()
	checkEgressAndLink(c3s2Eg[1], "C5-S1")
	checkEgressAndLink(c3s2Eg[2], "C5-S2")
}

func TestMsgTracingMissingOrigin(t *testing.T) {
	s, nc, sub := setupTest(t)
	defer s.Shutdown()
	defer nc.Close()

	// Produce the traces, but skip the trace from the origin serve.
	for i, tr := range traces {
		// Skip the trace for the origin server (i==1)
		if i != 1 {
			nc.Publish("trace.id", []byte(tr))
		}
	}
	e, err := GetMsgTrace(sub, "trace.id", 250*time.Millisecond, nil)
	// Since we are missing the origin, we will certainly have had a timeout.
	if err != nats.ErrTimeout {
		t.Fatalf("Should have got a timeout, got: %v", err)
	}
	// But the library should have added that missing trae with some
	// basic information (such as the server name that we got form
	// the other egress servers).
	if sn := e.Server.Name; sn != "C1-S1" {
		t.Fatalf("Expected server C1-S1, got %q", sn)
	}
	// We should have the ingress that indicates that the trace was missing
	ingress := e.Ingress()
	if ingress == nil {
		t.Fatal("Ingress is nil!")
	}
	if ie := ingress.Error; ie != ErrTraceMsgMissing {
		t.Fatalf("Expected ingress error, got %q", ie)
	}
	if ingress.Kind != srv.CLIENT {
		t.Fatalf("Expected ingress kind to be %v, got %v", srv.CLIENT, ingress.Kind)
	}
}

func TestMsgTracingMissingIntermediate(t *testing.T) {
	s, nc, sub := setupTest(t)
	defer s.Shutdown()
	defer nc.Close()

	// Produce the traces, but skip the trace from the origin serve.
	for i, tr := range traces {
		// Skip the trace from the C2-S1 gateway server (i==0)
		if i != 0 {
			nc.Publish("trace.id", []byte(tr))
		}
	}
	e, err := GetMsgTrace(sub, "trace.id", 250*time.Millisecond, nil)
	// Since we are missing some traces, we will certainly have had a timeout.
	if err != nats.ErrTimeout {
		t.Fatalf("Should have got a timeout, got: %v", err)
	}
	// The event we get should be from the origin server and be "C1-S1".
	if sn := e.Server.Name; sn != "C1-S1" {
		t.Fatalf("Expected server C1-S1, got %q", sn)
	}
	// From C1-S1, check the egress for the gateway, which is at pos 3
	c1s1Eg := e.Egresses()
	eg := c1s1Eg[3]
	// Make sure that eg.Kind was not changed (should be GATEWAY)
	if eg.Kind != srv.GATEWAY {
		t.Fatalf("Expected kind to be %v, got %v", srv.GATEWAY, eg.Kind)
	}
	gw := eg.Link
	if gw == nil {
		t.Fatal("Expected egress' link to be set, it was not")
	}
	if sn := gw.Server.Name; sn != "C2-S1" {
		t.Fatalf("Expected server name to be %q, got %q", "C2-S1", sn)
	}
	gwIng := gw.Ingress()
	if gwIng == nil {
		t.Fatal("Ingress is nil!")
	}
	if ie := gwIng.Error; ie != ErrTraceMsgMissing {
		t.Fatalf("Expected ingress error, got %q", ie)
	}
	if kind := gwIng.Kind; kind != srv.GATEWAY {
		t.Fatalf("Expected kind to be %v, got %v", srv.GATEWAY, kind)
	}
	egresses := gw.Egresses()
	if n := len(egresses); n != 1 {
		t.Fatalf("Expected 1 egress, got %v", n)
	}
	eg = egresses[0]
	if eg.Kind != srv.LEAF {
		t.Fatalf("Expected egress kind to be %v, got %v", srv.LEAF, eg.Kind)
	}
	if sn := eg.Name; sn != "C4-S1" {
		t.Fatalf("Expected egress name to be %q, got %q", "C4-S1", sn)
	}
}

func TestMsgTracingMissingSeveralChainnedTraces(t *testing.T) {
	s, nc, sub := setupTest(t)
	defer s.Shutdown()
	defer nc.Close()

	// For this test, we will suppose that there is something like:
	// S1 -> S2 -> S3 -> S4 -> S5, and will receive on S1 and S5.
	// S4 name can be reconstructed from S5's ingress and S2 from
	// S1's egress, but S3 should be marked as "<Unknown>".
	nc.Publish("trace.id", []byte(`{"server":{"name":"S1"},"request":{"header":{"Nats-Trace-Dest":["trace.id"]},"msgsize":50},"hops":1,"events":[{"type":"in","kind":0,"cid":13,"name":"Tracer","acc":"A","subj":"foo"},{"type":"eg","kind":4,"cid":14,"name":"S2","hop":"1"}]}`))
	nc.Publish("trace.id", []byte(`{"server":{"name":"S5"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["1.1.1.1"]},"msgsize":50},"events":[{"type":"in","kind":4,"cid":13,"name":"S4"}]}`))

	e, err := GetMsgTrace(sub, "trace.id", 250*time.Millisecond, nil)
	// Since we are missing some traces, we will certainly have had a timeout.
	if err != nats.ErrTimeout {
		t.Fatalf("Should have got a timeout, got: %v", err)
	}
	// The event we get should be from the origin server and be "S1".
	if sn := e.Server.Name; sn != "S1" {
		t.Fatalf("Expected server S1, got %q", sn)
	}
	// Ingress should not have the error
	ingress := e.Ingress()
	if ingress == nil {
		t.Fatalf("Ingress is nil!")
	}
	if ingress.Error != "" {
		t.Fatalf("Expected no ingress error, got %q", ingress.Error)
	}
	// From S1, check the egress, we should get S2, a LEAF
	s1Eg := e.Egresses()
	eg := s1Eg[0]
	if eg.Kind != srv.LEAF {
		t.Fatalf("Expected kind to be %v, got %v", srv.LEAF, eg.Kind)
	}

	s2 := eg.Link
	if s2 == nil {
		t.Fatal("Expected egress' link to be set, it was not")
	}
	if sn := s2.Server.Name; sn != "S2" {
		t.Fatalf("Expected server name to be %q, got %q", "S2", sn)
	}
	ingress = s2.Ingress()
	if ingress == nil {
		t.Fatalf("Ingress is nil!")
	}
	if ingress.Error != ErrTraceMsgMissing {
		t.Fatalf("Expected ingress error %q, got %q", ErrTraceMsgMissing, ingress.Error)
	}
	s2Eg := s2.Egresses()
	if n := len(s2Eg); n != 1 {
		t.Fatalf("Expected 1 egress, got %v", n)
	}
	eg = s2Eg[0]
	if eg.Kind != srv.LEAF {
		t.Fatalf("Expected egress' kind to be %v, got %v", srv.LEAF, eg.Kind)
	}
	if sn := eg.Name; sn != unknownServerName {
		t.Fatalf("Expected egress' name to be %q, got %q", unknownServerName, sn)
	}

	s3 := eg.Link
	if s3 == nil {
		t.Fatal("Expected egress' link to be set, it was not")
	}
	if sn := s3.Server.Name; sn != unknownServerName {
		t.Fatalf("Expected server name to be %q, got %q", unknownServerName, sn)
	}
	ingress = s3.Ingress()
	if ingress == nil {
		t.Fatalf("Ingress is nil!")
	}
	if ingress.Error != ErrTraceMsgMissing {
		t.Fatalf("Expected ingress error %q, got %q", ErrTraceMsgMissing, ingress.Error)
	}
	s3Eg := s3.Egresses()
	if n := len(s3Eg); n != 1 {
		t.Fatalf("Expected 1 egress, got %v", n)
	}
	eg = s3Eg[0]
	if eg.Kind != srv.LEAF {
		t.Fatalf("Expected egress' kind to be %v, got %v", srv.LEAF, eg.Kind)
	}
	if sn := eg.Name; sn != "S4" {
		t.Fatalf("Expected egress' name to be %q, got %q", "S4", sn)
	}

	s4 := eg.Link
	if s4 == nil {
		t.Fatal("Expected egress' link to be set, it was not")
	}
	if sn := s4.Server.Name; sn != "S4" {
		t.Fatalf("Expected server name to be %q, got %q", "S4", sn)
	}
	ingress = s4.Ingress()
	if ingress == nil {
		t.Fatalf("Ingress is nil!")
	}
	if ingress.Error != ErrTraceMsgMissing {
		t.Fatalf("Expected ingress error %q, got %q", ErrTraceMsgMissing, ingress.Error)
	}
	s4Eg := s4.Egresses()
	if n := len(s4Eg); n != 1 {
		t.Fatalf("Expected 1 egress, got %v", n)
	}
	eg = s4Eg[0]
	if eg.Kind != srv.LEAF {
		t.Fatalf("Expected egress' kind to be %v, got %v", srv.LEAF, eg.Kind)
	}
	if sn := eg.Name; sn != "S5" {
		t.Fatalf("Expected egress' name to be %q, got %q", "S5", sn)
	}

	s5 := eg.Link
	if s5 == nil {
		t.Fatal("Expected egress' link to be set, it was not")
	}
	if sn := s5.Server.Name; sn != "S5" {
		t.Fatalf("Expected server name to be %q, got %q", "S4", sn)
	}
	ingress = s5.Ingress()
	if ingress == nil {
		t.Fatalf("Ingress is nil!")
	}
	if ingress.Error != "" {
		t.Fatalf("Expected no ingress error, got %q", ingress.Error)
	}
	s5Eg := s5.Egresses()
	if n := len(s5Eg); n != 0 {
		t.Fatalf("Expected 0 egress, got %v", n)
	}
}

func TestMsgTracingTimeout(t *testing.T) {
	s, nc, sub := setupTest(t)
	defer s.Shutdown()
	defer nc.Close()

	// For this test, we will have S1 with 1 hop (S2), but get only trace from
	// S1. We should get a timeout error, but still get the trace from S1.
	nc.Publish("trace.id", []byte(`{"server":{"name":"S1"},"request":{"header":{"Nats-Trace-Dest":["trace.id"]},"msgsize":50},"hops":1,"events":[{"type":"in","kind":0,"cid":13,"name":"Tracer","acc":"A","subj":"foo"},{"type":"eg","kind":4,"cid":14,"name":"S2","hop":"1"}]}`))

	e, err := GetMsgTrace(sub, "trace.id", 500*time.Millisecond, nil)
	if err != nats.ErrTimeout {
		t.Fatalf("Expected timeout, but got %v", err)
	}
	if e == nil {
		t.Fatalf("Should still have had the S1 trace")
	}
	if sn := e.Server.Name; sn != "S1" {
		t.Fatalf("Expected server S1, got %q", sn)
	}
}

func TestMsgTracingGotThemAll(t *testing.T) {
	s, nc, sub := setupTest(t)
	defer s.Shutdown()
	defer nc.Close()

	// With this topology:
	//
	// C1-S1 <-- Route --> C1-S2 -- GW --> C2-S1 -- Leaf --> C4-S1
	//
	// Let's receive C4-S1 first, then C1-S2, then C1-S1 and finally C2-S1.
	//

	nc.Publish("trace.id", []byte(`{"server":{"name":"C4-S1","host":"0.0.0.0","id":"NDPYNVX6FK76IWBTOKI32CGPW263E7PSCX7QCTUVMRMLAAUYMXVJL3SL","cluster":"C4","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":16,"time":"2024-01-30T16:57:20.287804Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["2.1"]},"msgsize":71},"events":[{"type":"in","kind":4,"cid":6,"name":"C2-S1","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":7,"name":"sub6","sub":"foo"}]}`))
	nc.Publish("trace.id", []byte(`{"server":{"name":"C1-S2","host":"0.0.0.0","id":"NCPESRBATJ2BBJZOA2YNCC76VTHPH6ITORFR4H4CFULFCFIGMCIGRWOT","cluster":"C1","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":21,"time":"2024-01-30T16:57:20.287626Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["1"]},"msgsize":69},"events":[{"type":"in","kind":1,"cid":7,"name":"C1-S1","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":12,"name":"sub2","sub":"foo"}]}`))
	nc.Publish("trace.id", []byte(`{"server":{"name":"C1-S1","host":"0.0.0.0","id":"NBKYN6VUDABZLWVOWVFIUM7D63C7PWHS7TRWEQH2FR34ICEIAYBWJQS4","cluster":"C1","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":25,"time":"2024-01-30T16:57:20.287239Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"]},"msgsize":50},"hops":2,"events":[{"type":"in","kind":0,"cid":13,"name":"Tracer","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":14,"name":"sub1","sub":"foo"},{"type":"eg","kind":1,"cid":7,"name":"C1-S2","hop":"1"},{"type":"eg","kind":2,"cid":11,"name":"C2-S1","hop":"2"}]}`))
	nc.Publish("trace.id", []byte(`{"server":{"name":"C2-S1","host":"0.0.0.0","id":"NCL2I64LB5UHUVK6D6IX5GXZ3QFCYDLNRHHOUZBI2TGUO32TK747XUBW","cluster":"C2","ver":"2.11.0-dev","jetstream":false,"flags":0,"seq":25,"time":"2024-01-30T16:57:20.287475Z"},"request":{"header":{"Nats-Trace-Dest":["trace.id"],"Nats-Trace-Hop":["2"]},"msgsize":69},"hops":1,"events":[{"type":"in","kind":2,"cid":8,"name":"C1-S1","acc":"A","subj":"foo"},{"type":"eg","kind":0,"cid":10,"name":"sub3","sub":"foo"},{"type":"eg","kind":4,"cid":6,"name":"C4-S1","hop":"2.1"}]}`))

	e, err := GetMsgTrace(sub, "trace.id", 500*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("Error getting trace: %v", err)
	}
	if sn := e.Server.Name; sn != "C1-S1" {
		t.Fatalf("Expected server %q, got %q", "C1-S1", sn)
	}
	egresses := e.Egresses()
	if n := len(egresses); n != 3 {
		t.Fatalf("Expected 3 egresses, got %v", n)
	}
	for i, eg := range egresses {
		// Ignore the egress to CLIENT, which is the first in the list.
		if i == 0 {
			if eg.Kind != srv.CLIENT {
				t.Fatalf("Expected egress to a CLIENT, got %+v", eg)
			}
			continue
		}
		if eg.Link == nil {
			t.Fatal("Link is nil!")
		}
		switch i {
		case 1:
			if sn := eg.Link.Server.Name; sn != "C1-S2" {
				t.Fatalf("Expected link to %q, got %q", "C1-S2", sn)
			}
			egs := eg.Link.Egresses()
			if n := len(egs); n != 1 {
				t.Fatalf("Expected 1 egress from C2-S1, got %v", n)
			}
			if e := egs[0]; e.Kind != srv.CLIENT {
				t.Fatalf("Expected egress to a CLIENT, got %+v", e)
			}
		case 2:
			if sn := eg.Link.Server.Name; sn != "C2-S1" {
				t.Fatalf("Expected link to %q, got %q", "C2-S1", sn)
			}
			egs := eg.Link.Egresses()
			if n := len(egs); n != 2 {
				t.Fatalf("Expected 2 egresses from C2-S1, got %v", n)
			}
			// We ignore the egress to the client and check the one to the LEAF
			gweg := egs[1]
			if gweg.Link == nil {
				t.Fatalf("Link to %q is nil!", "C4-S1")
			}
			if sn := gweg.Link.Server.Name; sn != "C4-S1" {
				t.Fatalf("Expected C2-S1 egress to be %q, got %q", "C4-S1", sn)
			}
			ln := gweg.Link
			if n := len(ln.Egresses()); n != 1 {
				t.Fatalf("Expected 1 egress from C4-S1, got %v", n)
			}
			if e := ln.Egresses()[0]; e.Kind != srv.CLIENT {
				t.Fatalf("Expected egress to a CLIENT, got %+v", e)
			}
		}
	}
}
