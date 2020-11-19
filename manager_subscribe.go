package jsm

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

type subopts struct {
	streamName  string
	durableName string
	consumer    *api.ConsumerConfig
	cb          nats.MsgHandler
	mgr         *Manager
	timeout     time.Duration
	inbox       string
	sync        bool
}

func newSubOpts(mgr *Manager, opts []SubscribeOption) (*subopts, error) {
	if mgr == nil {
		return nil, fmt.Errorf("not bound to a Jetstream Manager")
	}

	cfg := mgr.dfltConsumer
	so := &subopts{mgr: mgr, consumer: &cfg, timeout: mgr.timeout}

	for _, opt := range opts {
		err := opt(so)
		if err != nil {
			return nil, fmt.Errorf("invalid options: %s", err)
		}
	}

	return so, nil
}

type Subscription struct {
	sub      *nats.Subscription
	sync     bool
	consumer *Consumer
	timeout  time.Duration
}

func (s *Subscription) Consumer() *Consumer {
	return s.consumer
}

func (s *Subscription) NextMsg() (*nats.Msg, error) {
	if s == nil || s.consumer == nil || s.consumer.mgr == nil || s.consumer.mgr.nc == nil {
		return nil, fmt.Errorf("invalid subscription")
	}

	if s.consumer.IsPushMode() {
		if s.sub == nil {
			return nil, fmt.Errorf("invalid subscription")
		}

		if !s.sync {
			return nil, fmt.Errorf("not a synchronous subscription")
		}

		return s.sub.NextMsg(s.timeout)
	}

	req := &api.JSApiConsumerGetNextRequest{Expires: time.Now().Add(s.timeout), Batch: 1}
	jreq, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	msg, err := s.consumer.mgr.nc.Request(s.consumer.NextSubject(), jreq, s.timeout)
	if err != nil {
		return nil, err
	}

	return msg, err
}

func (s *Subscription) Unsubscribe() error {
	if s.sub == nil {
		return fmt.Errorf("invalid subscription")
	}

	return s.sub.Unsubscribe()
}

type SubscribeOption func(*subopts) error

// SubscribeStream specifies the stream name the subject belongs to and results in fewer API calls when subscribing
func SubscribeStream(stream string) SubscribeOption {
	return func(o *subopts) error {
		o.streamName = stream
		return nil
	}
}

// SubscribeConsumer creates a consumer based on the manager default consumer
func SubscribeConsumer(opts ...ConsumerOption) SubscribeOption {
	return func(o *subopts) error {
		if o.mgr == nil {
			return fmt.Errorf("not bound to a Jetstream Manager")
		}

		var err error
		o.consumer, err = NewConsumerConfiguration(o.mgr.dfltConsumer, opts...)
		if err != nil {
			return err
		}

		return nil
	}
}

// SubscribeHandler creates an asynchronous subscription where each received message is processed by cb, not usable in Pull mode
func SubscribeHandler(cb nats.MsgHandler) SubscribeOption {
	return func(o *subopts) error {
		o.cb = cb
		return nil
	}
}

// SubscribeTimeout timeout when waiting for new messages and acknowledgements
func SubscribeTimeout(timeout time.Duration) SubscribeOption {
	return func(o *subopts) error {
		o.timeout = timeout
		return nil
	}
}

func SubscribeDurable(name string) SubscribeOption {
	return func(o *subopts) error {
		if !IsValidName(name) {
			return fmt.Errorf("invalid durable name")
		}

		o.durableName = name
		return nil
	}
}

func (m *Manager) subscribeEphemeral(subject string, sopts *subopts) (*Subscription, error) {
	var err error

	if sopts.consumer == nil {
		return nil, fmt.Errorf("no default consumer template")
	}

	if !sopts.sync && sopts.cb == nil {
		sopts.sync = true
	}

	if sopts.sync && sopts.cb != nil {
		return nil, fmt.Errorf("message handler in sync mode is not supported")
	}

	sopts.inbox = nats.NewInbox()

	cfg := *sopts.consumer
	cfg.Durable = ""
	cfg.DeliverSubject = sopts.inbox
	cfg.FilterSubject = subject

	subs := &Subscription{timeout: sopts.timeout, sync: sopts.sync}

	// create interest before the durable
	if sopts.sync {
		subs.sub, err = m.nc.SubscribeSync(sopts.inbox)
		if err != nil {
			return nil, err
		}
	} else {
		subs.sub, err = m.nc.Subscribe(sopts.inbox, sopts.cb)
		if err != nil {
			return nil, err
		}
	}

	subs.consumer, err = m.NewConsumerFromDefault(sopts.streamName, cfg)
	if err != nil {
		return nil, err
	}

	return subs, nil
}

func (m *Manager) subscribeDurable(subject string, sopts *subopts) (*Subscription, error) {
	if sopts.sync && sopts.cb != nil {
		return nil, fmt.Errorf("message handler in sync mode is not supported")
	}

	if !sopts.sync && sopts.cb == nil {
		return nil, fmt.Errorf("message handler is required in async mode")
	}

	subs := &Subscription{timeout: sopts.timeout, sync: sopts.sync}
	var err error

	cc, _ := m.LoadConsumer(sopts.streamName, sopts.durableName)
	if cc == nil {
		cfg := *sopts.consumer
		cfg.Durable = sopts.durableName
		cfg.FilterSubject = subject

		switch {
		case sopts.sync && cfg.DeliverSubject != "":
			// force sync
			cfg.DeliverSubject = ""
		case !sopts.sync && cfg.DeliverSubject == "":
			cfg.DeliverSubject = nats.NewInbox()
		}
		sopts.inbox = cfg.DeliverSubject

		cc, err = m.NewConsumerFromDefault(sopts.streamName, cfg)
		if err != nil {
			return nil, err
		}
	}

	if !sopts.sync && cc.IsPullMode() {
		return nil, fmt.Errorf("pull mode consumers require sync subscriptions")
	}

	subs.consumer = cc

	if sopts.inbox == "" && cc.IsPushMode() {
		// we didn't specifically set the subject when creating it, so we update it to us
		cfg := cc.Configuration()
		cfg.DeliverSubject = nats.NewInbox()

		subs.consumer, err = m.NewConsumerFromDefault(sopts.streamName, cfg)
		if err != nil {
			return nil, err
		}
	}

	sopts.inbox = subs.consumer.DeliverySubject()

	switch {
	case sopts.inbox == "":
		// pull

	case sopts.cb != nil && sopts.inbox != "":
		subs.sub, err = m.nc.Subscribe(sopts.inbox, sopts.cb)

	case sopts.cb == nil && sopts.inbox != "":
		subs.sync = true
		subs.sub, err = m.nc.SubscribeSync(sopts.inbox)

	default:
		return nil, fmt.Errorf("unsupported scenario")
	}
	if err != nil {
		return nil, err
	}

	return subs, err
}

func (m *Manager) SubscribeSync(subject string, opts ...SubscribeOption) (*Subscription, error) {
	return m.subscribe(subject, true, opts...)
}

func (m *Manager) Subscribe(subject string, opts ...SubscribeOption) (*Subscription, error) {
	return m.subscribe(subject, false, opts...)
}

func (m *Manager) subscribe(subject string, sync bool, opts ...SubscribeOption) (*Subscription, error) {
	sopts, err := newSubOpts(m, opts)
	if err != nil {
		return nil, err
	}
	sopts.sync = sync

	if sopts.sync && sopts.durableName == "" {
		return nil, fmt.Errorf("pull consumers require a durable name")
	}

	// figure out the stream name, user can avoid api access using SubscribeStream()
	if sopts.streamName == "" {
		if subject == "" {
			return nil, fmt.Errorf("subject or SubscribeStream() is required")
		}

		sopts.streamName, err = m.StreamLookup(subject)
		if err != nil || sopts.streamName == "" {
			return nil, fmt.Errorf("could not find a stream with interest on %q", subject)
		}
	}

	if sopts.durableName == "" {
		sub, err := m.subscribeEphemeral(subject, sopts)
		if err != nil {
			return nil, fmt.Errorf("ephemeral subscribe failed: %s", err)
		}

		return sub, nil
	}

	sub, err := m.subscribeDurable(subject, sopts)
	if err != nil {
		return nil, fmt.Errorf("durable subscribe failed: %s", err)
	}

	return sub, nil
}
