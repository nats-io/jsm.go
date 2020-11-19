package jsm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

type pubopts struct {
	expectStreams   []string
	expectAnyStream bool
	timeout         time.Duration
	stream          *api.StreamConfig
	mgr             *Manager
}

func newPubOpts(mgr *Manager, opts []PublishOption) (*pubopts, error) {
	if mgr == nil {
		return nil, fmt.Errorf("not bound to a Jetstream Manager")
	}

	po := &pubopts{mgr: mgr}

	for _, opt := range opts {
		err := opt(po)
		if err != nil {
			return nil, fmt.Errorf("invalid options: %s", err)
		}
	}

	return po, nil
}

// PublishOption configures JetStream aware publishing
type PublishOption func(o *pubopts) error

// PublishResult is the result from JetStream when publish acknowledgment is requested
type PublishResult struct {
	*api.PubAck
}

// PublishExpectStream verifies that the publish reached the correct stream, when empty any success JetStream response is accepted, else the responding stream has to be one of the listed ones
func PublishExpectStream(streams ...string) PublishOption {
	return func(o *pubopts) error {
		if o.timeout == 0 {
			if o.mgr == nil {
				o.timeout = 2 * time.Second
			} else {
				o.timeout = o.mgr.timeout
			}
		}

		if len(streams) == 0 {
			o.expectAnyStream = true
			return nil
		}

		o.expectStreams = append(o.expectStreams, streams...)
		return nil
	}
}

// PublishTimeout waits this long for a response from JetStream, if implies PublishExpectStream() if not specifically set
func PublishTimeout(d time.Duration) PublishOption {
	return func(o *pubopts) error {
		o.timeout = d

		if len(o.expectStreams) == 0 {
			err := PublishExpectStream()(o)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// PublishMsg publishes a nats Msg to JetStream with optional publish ack and on-demand stream creation
func (m *Manager) PublishMsg(msg *nats.Msg, opts ...PublishOption) (*PublishResult, error) {
	return m.publish("", nil, msg, opts...)
}

// PublishMsg publishes data to JetStream with optional publish ack and on-demand stream creation
func (m *Manager) Publish(subject string, data []byte, opts ...PublishOption) (*PublishResult, error) {
	return m.publish(subject, data, nil, opts...)
}

// TODO: figure out request, JS does not store reply though

func (m *Manager) publish(subject string, data []byte, msg *nats.Msg, opts ...PublishOption) (*PublishResult, error) {
	var err error

	popts, err := newPubOpts(m, opts)
	if err != nil {
		return nil, err
	}

	if subject == "" && msg != nil {
		subject = msg.Subject
	}

	var resp = &PublishResult{}

	if popts.expectAnyStream || len(popts.expectStreams) > 0 || popts.timeout > 0 {
		var rm *nats.Msg

		if msg != nil {
			rm, err = m.nc.RequestMsg(msg, popts.timeout)
		} else {
			rm, err = m.nc.Request(subject, data, popts.timeout)
		}
		if err != nil {
			return nil, err
		}

		if IsErrorResponse(rm) {
			return nil, ParseErrorResponse(rm)
		}

		rdata := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(string(rm.Data), api.OK), " '"), "'"))
		err = json.Unmarshal([]byte(rdata), resp)
		if err != nil {
			return nil, fmt.Errorf("invalid response from Jetstream: %s", err)
		}

		if popts.expectAnyStream {
			if resp.Stream == "" {
				return resp, fmt.Errorf("did not receive a valid response from any stream")
			}
		}

		if len(popts.expectStreams) > 0 {
			found := false
			for _, e := range popts.expectStreams {
				if e == resp.Stream {
					found = true
					break
				}
			}

			if !found {
				return resp, fmt.Errorf("response from stream %q is not one of the expected streams", resp.Stream)
			}
		}

		return resp, nil
	}

	return resp, m.nc.Publish(subject, data)
}
