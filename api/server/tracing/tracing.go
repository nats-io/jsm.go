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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/s2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	ErrTraceMsgMissing    = "This trace event was missing"
	unknownServerName     = "<Unknown>"
	contentEncodingHeader = "Content-Encoding"
)

// TraceMsg copies `traceMsg` and sets up the subscription `GetMsgTrace` needs,
// adds needed headers and publish the message before calling `GetMsgTrace`.
//
// If `deliverToDest` is false the message will not reach the target subject
// but the trace will include details as if it would have
func TraceMsg(nc *nats.Conn, traceMsg *nats.Msg, deliverToDest bool, timeout time.Duration, trace chan *nats.Msg) (*server.MsgTraceEvent, error) {
	sub, err := nc.SubscribeSync(nc.NewRespInbox())
	if err != nil {
		return nil, err
	}
	defer sub.Unsubscribe()

	err = nc.Flush()
	if err != nil {
		return nil, err
	}

	time.Sleep(250 * time.Millisecond) // allow for super cluster propagation

	newMsg := nats.NewMsg(traceMsg.Subject)

	newMsg.Data = make([]byte, len(traceMsg.Data))
	copy(newMsg.Data, traceMsg.Data)

	for k, vs := range traceMsg.Header {
		for _, v := range vs {
			newMsg.Header.Add(k, v)
		}
	}
	newMsg.Header.Set("Accept-Encoding", "snappy")
	newMsg.Header.Set(server.MsgTraceDest, sub.Subject)
	if !deliverToDest {
		newMsg.Header.Set(server.MsgTraceOnly, "true")
	}

	err = nc.PublishMsg(newMsg)
	if err != nil {
		return nil, err
	}

	return GetMsgTrace(sub, sub.Subject, timeout, trace)
}

// GetMsgTrace returns the message traces that were sent by the
// NATS server(s) on the given `traceSubject`. A single `srv.MsgTraceEvent`
// object is returned. If the message went to different servers, the
// trace of these servers can be found in the corresponding `Egress`
// events of the returned trace.
//
// Note that even if an error is returned (for instance a timeout if
// not all expected trace messages have been received), this call will
// return a `srv.MsgTraceEvent` that can still be inspected. Of course,
// if no trace at all is received, the returned `srv.MsgTraceEvent` will
// be nil.
//
// The given `timeout` is used for each call to `sub.NextMsg()`, which
// means that the overall duration of this call can be overall larger
// than the timeout.
//
// The caller is expected to have created a synchronous subscription,
// typically on a wildcard subject, say "trace.*", and send a message
// with the `Nats-Trace-Dest` header set to an unique subject, say
// `trace.id1`. This destination is then passed to `GetMsgTrace()`.
// The subscription should be used by a single go routine since
// traces that do not match the `traceSubject` subject but received
// by the subscription will be dropped.
func GetMsgTrace(sub *nats.Subscription, traceSubject string, timeout time.Duration, trace chan *nats.Msg) (*server.MsgTraceEvent, error) {
	var (
		origin  *server.MsgTraceEvent
		traces  = map[string]*server.MsgTraceEvent{}
		missing = map[string]*server.MsgTraceEvent{}
		servers = map[string]*server.MsgTraceEvent{}
	)

	var retErr error
	setErr := func(err error) {
		if retErr != nil {
			return
		}
		retErr = err
	}

	for {
		msg, err := sub.NextMsg(timeout)
		// If an error here, we break the loop, for other errors inside this
		// for-loop, we will continue.
		if err != nil {
			setErr(err)
			break
		}
		// Drop this message if not for our current trace message.
		if msg.Subject != traceSubject {
			continue
		}
		// This will uncompress if needed, otherwise, it is simply msg.Data.
		data, err := getData(msg)
		if err != nil {
			setErr(err)
			continue
		}

		// save the decompressed data into the raw trace
		if trace != nil {
			msg.Data = data
			trace <- msg
		}

		var e *server.MsgTraceEvent
		if err = json.Unmarshal(data, &e); err != nil {
			setErr(err)
			continue
		}
		ingress := e.Ingress()
		if ingress == nil {
			setErr(fmt.Errorf("missing ingress in trace event: %+v", e))
			continue
		}
		// Is this the trace event from the origin server, that is, the one
		// with an ingress.Kind == CLIENT? There should be only one.
		if ingress.Kind == server.CLIENT {
			if origin != nil {
				setErr(fmt.Errorf("duplicate ingress from client: %+v", ingress))
				continue
			}
			// Save our origin event.
			origin = e
		} else {
			// Save this event in the traces map. We use "Hop" header as the key.
			hop := nats.Header(e.Request.Header).Get(server.MsgTraceHop)
			if hop == "" {
				setErr(fmt.Errorf("event for this remote should have a 'Hop' header, but it did not: %+v", e))
				continue
			}
			// Add to servers map.
			traces[hop] = e
		}
		// Add the trace of this server in the map
		servers[e.Server.Name] = e
		// Check if we got all the expected traces...
		if gotAllServers(servers, origin != nil) {
			break
		}
	}
	// We may have got an error, but we are still going to stitch all the events
	// we got together, creating missing MsgTraceEvent traces.
	//
	// First order the 'hop' of the traces we got.
	sorted := make([]string, 0, len(traces))
	for k := range traces {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, hop := range sorted {
		// This call will make sure that "parent" of the current trace event
		// exists, if not will create it and add this trace as its egress.
		// If there was no origin, and this call creates it, it will be
		// returned.
		origin = ensureParentExists(hop, origin, traces, missing)
	}
	// Link all events together
	linkEgresses(origin, traces)

	return origin, retErr
}

func gotAllServers(all map[string]*server.MsgTraceEvent, hasOrigin bool) bool {
	if !hasOrigin {
		return false
	}
	for _, tr := range all {
		// We already made sure that we have an ingress, so no need to check
		// for `nil` here.
		in := tr.Ingress()
		// If this is the ingress from the CLIENT, skip this check
		if in.Kind != server.CLIENT {
			// Do we have the ingress server in the `all` map? If not, then
			// clearly we don't have them all.
			if _, ok := all[in.Name]; !ok {
				return false
			}
		}
		// If this trace has "hops" count, then check the egress to see if
		// we have the trace for those servers.
		if tr.Hops > 0 {
			egresses := tr.Egresses()
			for _, eg := range egresses {
				if eg.Kind == server.CLIENT {
					continue
				}
				// If we still don't have this egress server, then we don't
				// have them all.
				if _, ok := all[eg.Name]; !ok {
					return false
				}
			}
		}
	}
	return true
}

func getData(m *nats.Msg) ([]byte, error) {
	data := m.Data
	eh := m.Header.Get(contentEncodingHeader)
	switch eh {
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		data, err = io.ReadAll(zr)
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, err
		}
		if err = zr.Close(); err != nil {
			return nil, err
		}
	case "snappy":
		var err error
		sr := s2.NewReader(bytes.NewReader(data))
		data, err = io.ReadAll(sr)
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, err
		}
	}
	return data, nil
}

func ensureParentExists(hop string, origin *server.MsgTraceEvent, traces, missing map[string]*server.MsgTraceEvent) *server.MsgTraceEvent {
	if hop == "" {
		return origin
	}

	var parentHop string
	var parentTrace *server.MsgTraceEvent

	// This is the trace we are dealing with, the "child" and we will make
	// sure that the "parent" is found, or create one if needed.
	child := traces[hop]
	ingress := child.Ingress()
	pos := strings.LastIndexByte(hop, '.')
	if pos != -1 {
		parentHop = hop[:pos]
		parentTrace = traces[parentHop]
	} else {
		parentTrace = origin
	}
	if parentTrace == nil {
		parentTrace = addMissingTraceEvent(ingress, hop, child, pos == -1)
		missing[parentHop] = parentTrace
		if pos == -1 {
			origin = parentTrace
		} else {
			traces[parentHop] = parentTrace
		}
	} else if _, parentWasMissing := missing[parentHop]; parentWasMissing {
		// If the parent trace was missing, we need to add an Egress for this child trace.
		egresses := parentTrace.Egresses()
		found := false
		for _, eg := range egresses {
			if eg.Name == child.Server.Name {
				found = true
				break
			}
		}
		if !found {
			parentTrace.Events = append(parentTrace.Events, &server.MsgTraceEgress{
				MsgTraceBase: server.MsgTraceBase{Type: server.MsgTraceEgressType},
				Kind:         ingress.Kind, // The egress' kind for the parent is the ingress' kind of the child.
				Name:         child.Server.Name,
				Hop:          hop,
			})
		}
	}
	return ensureParentExists(parentHop, origin, traces, missing)
}

func addMissingTraceEvent(ingress *server.MsgTraceIngress, hop string, child *server.MsgTraceEvent, parentIsOrigin bool) *server.MsgTraceEvent {
	// We are going to create a MsgTraceEvent that represents the parent of
	// the given child. The parent information is given by the child's ingress.
	// We add an ingress with the error but also the kind so that if we are
	// missing several chainned traces, we don't set the kind to 0 (CLIENT)
	// but use the last known one.
	ikind := ingress.Kind
	// But if this missing is the origin, set the ingress's kind to CLIENT
	if parentIsOrigin {
		ikind = server.CLIENT
	}
	in := &server.MsgTraceIngress{
		MsgTraceBase: server.MsgTraceBase{Type: server.MsgTraceIngressType},
		Kind:         ikind,
		Error:        ErrTraceMsgMissing,
	}
	// We will add and Egress for this child.
	eg := &server.MsgTraceEgress{
		MsgTraceBase: server.MsgTraceBase{Type: server.MsgTraceEgressType},
		Kind:         ingress.Kind, // The egress' kind for the parent is the ingress' kind of the child.
		Name:         child.Server.Name,
		Hop:          hop,
	}
	// Again, if we are missing several chainned servers, it is possible that
	// the ingress.Name is empty, in this case replace with <Unknown>.
	srvName := ingress.Name
	if srvName == "" {
		srvName = unknownServerName
	}
	return &server.MsgTraceEvent{
		Server: server.ServerInfo{Name: srvName},
		Events: []server.MsgTrace{in, eg},
	}
}

func linkEgresses(e *server.MsgTraceEvent, m map[string]*server.MsgTraceEvent) {
	if e == nil {
		return
	}
	ingress := e.Ingress()
	egresses := e.Egresses()
	for _, eg := range egresses {
		if eg.Kind == server.CLIENT {
			continue
		}
		// We should have a Hop field set, which will be the key that
		// is present in the map.
		k := eg.Hop
		// If the link is not found, having eg.Link == nil will be an indication
		// that we did not receive the message trace from this rmote.
		if link, ok := m[k]; ok {
			delete(m, k)
			eg.Link = link
			// If this trace was not missing (no error in ingress.Error)
			// and the link was missing (error in linkIngress.Error),
			// then possibly fixup the link info such as server name
			// and ingress' kind.
			if ingress.Error != ErrTraceMsgMissing {
				linkIngress := link.Ingress()
				if linkIngress.Error == ErrTraceMsgMissing {
					if eg.Name != link.Server.Name {
						link.Server.Name = eg.Name
					}
					if eg.Kind != linkIngress.Kind {
						linkIngress.Kind = eg.Kind
					}
				}
			}
			// Now recursively process this remote.
			linkEgresses(link, m)
		}
	}
}
