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

package jsm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// DefaultConsumer is the configuration that will be used to create new Consumers in NewConsumer
var DefaultConsumer = server.ConsumerConfig{
	DeliverAll:   true,
	AckPolicy:    server.AckExplicit,
	AckWait:      30 * time.Second,
	ReplayPolicy: server.ReplayInstant,
}

// SampledDefaultConsumer is the configuration that will be used to create new Consumers in NewConsumer
var SampledDefaultConsumer = server.ConsumerConfig{
	DeliverAll:      true,
	AckPolicy:       server.AckExplicit,
	AckWait:         30 * time.Second,
	ReplayPolicy:    server.ReplayInstant,
	SampleFrequency: "100%",
}

// ConsumerOptions configures consumers
type ConsumerOption func(o *ConsumerConfig) error

// Consumer represents a JetStream consumer
type Consumer struct {
	name   string
	stream string
	cfg    *ConsumerConfig
}

type ConsumerConfig struct {
	server.ConsumerConfig

	conn  *reqoptions
	ropts []RequestOption
}

// NewConsumerFromDefault creates a new consumer based on a template config that gets modified by opts
func NewConsumerFromDefault(stream string, dflt server.ConsumerConfig, opts ...ConsumerOption) (consumer *Consumer, err error) {
	cfg, err := NewConsumerConfiguration(dflt, opts...)
	if err != nil {
		return nil, err
	}

	req := server.CreateConsumerRequest{
		Stream: stream,
		Config: cfg.ConsumerConfig,
	}

	var createdName string

	switch req.Config.Durable {
	case "":
		createdName, err = createEphemeralConsumer(req, cfg.conn)
	default:
		createdName, err = createDurableConsumer(req, cfg.conn)
	}
	if err != nil {
		return nil, err
	}

	if createdName == "" {
		return nil, fmt.Errorf("expected a consumer name but none were generated")
	}

	return LoadConsumer(stream, createdName, cfg.ropts...)
}

func createDurableConsumer(req server.CreateConsumerRequest, opts *reqoptions) (name string, err error) {
	jreq, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	_, err = request(fmt.Sprintf(server.JetStreamCreateConsumerT, req.Stream, req.Config.Durable), jreq, opts)
	if err != nil {
		return "", err
	}

	return req.Config.Durable, nil
}

func createEphemeralConsumer(req server.CreateConsumerRequest, opts *reqoptions) (name string, err error) {
	jreq, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	response, err := request(fmt.Sprintf(server.JetStreamCreateEphemeralConsumerT, req.Stream), jreq, opts)
	if err != nil {
		return "", err
	}

	parts := strings.Split(string(response.Data), " ")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ephemeral OK response from server: %q", response.Data)
	}

	return parts[1], nil
}

// NewConsumer creates a consumer based on DefaultConsumer modified by opts
func NewConsumer(stream string, opts ...ConsumerOption) (consumer *Consumer, err error) {
	return NewConsumerFromDefault(stream, DefaultConsumer, opts...)
}

// LoadOrNewConsumer loads a consumer by name if known else creates a new one with these properties
func LoadOrNewConsumer(stream string, name string, opts ...ConsumerOption) (consumer *Consumer, err error) {
	return LoadOrNewConsumerFromDefault(stream, name, DefaultConsumer, opts...)
}

// LoadOrNewConsumerFromDefault loads a consumer by name if known else creates a new one with these properties based on template
func LoadOrNewConsumerFromDefault(stream string, name string, template server.ConsumerConfig, opts ...ConsumerOption) (consumer *Consumer, err error) {
	cfg, err := NewConsumerConfiguration(DefaultConsumer, opts...)
	if err != nil {
		return nil, err
	}

	c, err := LoadConsumer(stream, name, cfg.ropts...)
	if c == nil || err != nil {
		return NewConsumerFromDefault(stream, template, opts...)
	}

	return c, err
}

// LoadConsumer loads a consumer by name
func LoadConsumer(stream string, name string, opts ...RequestOption) (consumer *Consumer, err error) {
	conn, err := newreqoptions(opts...)
	if err != nil {
		return nil, err
	}

	consumer = &Consumer{
		name:   name,
		stream: stream,
		cfg: &ConsumerConfig{
			ConsumerConfig: server.ConsumerConfig{},
			conn:           conn,
			ropts:          opts,
		},
	}

	err = loadConfigForConsumer(consumer)
	if err != nil {
		return nil, err
	}

	return consumer, nil
}

// NewConsumerConfiguration generates a new configuration based on template modified by opts
func NewConsumerConfiguration(dflt server.ConsumerConfig, opts ...ConsumerOption) (*ConsumerConfig, error) {
	cfg := &ConsumerConfig{
		ConsumerConfig: dflt,
		conn:           dfltreqoptions(),
	}

	for _, o := range opts {
		err := o(cfg)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func loadConfigForConsumer(consumer *Consumer) (err error) {
	info, err := loadConsumerInfo(consumer.stream, consumer.name, consumer.cfg.conn)
	if err != nil {
		return err
	}

	consumer.cfg.ConsumerConfig = info.Config

	return nil
}

func loadConsumerInfo(s string, c string, opts *reqoptions) (info server.ConsumerInfo, err error) {
	response, err := request(fmt.Sprintf(server.JetStreamConsumerInfoT, s, c), nil, opts)
	if err != nil {
		return info, err
	}

	info = server.ConsumerInfo{}
	err = json.Unmarshal(response.Data, &info)
	if err != nil {
		return info, err
	}

	return info, nil
}

func DeliverySubject(s string) ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.Delivery = s
		return nil
	}
}

func DurableName(s string) ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.Durable = s
		return nil
	}
}

func StartAtSequence(s uint64) ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.StreamSeq = s
		return nil
	}
}

func StartAtTime(t time.Time) ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.StartTime = t
		return nil
	}
}

func DeliverAllAvailable() ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.DeliverAll = true
		return nil
	}
}

func StartWithLastReceived() ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.DeliverLast = true
		return nil
	}
}

func StartAtTimeDelta(d time.Duration) ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.StartTime = time.Now().Add(-1 * d)
		return nil
	}
}

func AcknowledgeNone() ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.AckPolicy = server.AckNone
		return nil
	}
}

func AcknowledgeAll() ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.AckPolicy = server.AckAll
		return nil
	}
}

func AcknowledgeExplicit() ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.AckPolicy = server.AckExplicit
		return nil
	}
}

func AckWait(t time.Duration) ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.AckWait = t
		return nil
	}
}

func MaxDeliveryAttempts(n int) ConsumerOption {
	return func(o *ConsumerConfig) error {
		if n == 0 {
			return fmt.Errorf("configuration would prevent all deliveries")
		}
		o.ConsumerConfig.MaxDeliver = n
		return nil
	}
}

func FilterStreamBySubject(s string) ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.FilterSubject = s
		return nil
	}
}

func ReplayInstantly() ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.ReplayPolicy = server.ReplayInstant
		return nil
	}
}

func ReplayAsReceived() ConsumerOption {
	return func(o *ConsumerConfig) error {
		o.ConsumerConfig.ReplayPolicy = server.ReplayOriginal
		return nil
	}
}

func SamplePercent(i int) ConsumerOption {
	return func(o *ConsumerConfig) error {
		if i < 0 || i > 100 {
			return fmt.Errorf("sample percent must be 0-100")
		}

		if i == 0 {
			o.ConsumerConfig.SampleFrequency = ""
			return nil
		}

		o.ConsumerConfig.SampleFrequency = fmt.Sprintf("%d%%", i)
		return nil
	}
}

func ConsumerConnection(opts ...RequestOption) ConsumerOption {
	return func(o *ConsumerConfig) error {
		for _, opt := range opts {
			opt(o.conn)
		}

		o.ropts = append(o.ropts, opts...)

		return nil
	}
}

// Reset reloads the Consumer configuration from the JetStream server
func (c *Consumer) Reset() error {
	return loadConfigForConsumer(c)
}

// NextSubject returns the subject used to retrieve the next message for pull-based Consumers, empty when not a pull-base consumer
func (c *Consumer) NextSubject() string {
	if !c.IsPullMode() {
		return ""
	}

	s, _ := NextSubject(c.stream, c.name)

	return s
}

// NextSubject returns the subject used to retrieve the next message for pull-based Consumers, empty when not a pull-base consumer
func NextSubject(stream string, consumer string) (string, error) {
	if stream == "" {
		return "", fmt.Errorf("stream name can not be empty string")
	}

	if consumer == "" {
		return "", fmt.Errorf("consumer name can not be empty string")
	}

	return fmt.Sprintf(server.JetStreamRequestNextT, stream, consumer), nil
}

// AckSampleSubject is the subject used to publish ack samples to
func (c *Consumer) AckSampleSubject() string {
	if c.SampleFrequency() == "" {
		return ""
	}

	return server.JetStreamMetricConsumerAckPre + "." + c.StreamName() + "." + c.name
}

// AdvisorySubject is a wildcard subscription subject that subscribes to all advisories for this consumer
func (c *Consumer) AdvisorySubject() string {
	return server.JetStreamAdvisoryPrefix + "." + "*" + "." + c.StreamName() + "." + c.name
}

// MetricSubject is a wildcard subscription subject that subscribes to all metrics for this consumer
func (c *Consumer) MetricSubject() string {
	return server.JetStreamMetricPrefix + "." + "*" + "." + c.StreamName() + "." + c.name
}

// Subscribe see nats.Subscribe
func (c *Consumer) Subscribe(h func(*nats.Msg)) (sub *nats.Subscription, err error) {
	if !c.IsPushMode() {
		return nil, fmt.Errorf("consumer %s > %s is not push-based", c.stream, c.name)
	}

	return c.cfg.conn.nc.Subscribe(c.DeliverySubject(), h)
}

// ChanSubscribe see nats.ChangSubscribe
func (c *Consumer) ChanSubscribe(ch chan *nats.Msg) (sub *nats.Subscription, err error) {
	if !c.IsPushMode() {
		return nil, fmt.Errorf("consumer %s > %s is not push-based", c.stream, c.name)
	}

	return c.cfg.conn.nc.ChanSubscribe(c.DeliverySubject(), ch)
}

// ChanQueueSubscribe see nats.ChanQueueSubscribe
func (c *Consumer) ChanQueueSubscribe(group string, ch chan *nats.Msg) (sub *nats.Subscription, err error) {
	if !c.IsPushMode() {
		return nil, fmt.Errorf("consumer %s > %s is not push-based", c.stream, c.name)
	}

	return c.cfg.conn.nc.ChanQueueSubscribe(c.DeliverySubject(), group, ch)
}

// SubscribeSync see nats.SubscribeSync
func (c *Consumer) SubscribeSync() (sub *nats.Subscription, err error) {
	if !c.IsPushMode() {
		return nil, fmt.Errorf("consumer %s > %s is not push-based", c.stream, c.name)
	}

	return c.cfg.conn.nc.SubscribeSync(c.DeliverySubject())
}

// QueueSubscribe see nats.QueueSubscribe
func (c *Consumer) QueueSubscribe(queue string, h func(*nats.Msg)) (sub *nats.Subscription, err error) {
	if !c.IsPushMode() {
		return nil, fmt.Errorf("consumer %s > %s is not push-based", c.stream, c.name)
	}

	return c.cfg.conn.nc.QueueSubscribe(c.DeliverySubject(), queue, h)
}

// QueueSubscribeSync see nats.QueueSubscribeSync
func (c *Consumer) QueueSubscribeSync(queue string) (sub *nats.Subscription, err error) {
	if !c.IsPushMode() {
		return nil, fmt.Errorf("consumer %s > %s is not push-based", c.stream, c.name)
	}

	return c.cfg.conn.nc.QueueSubscribeSync(c.DeliverySubject(), queue)
}

// QueueSubscribeSyncWithChan see nats.QueueSubscribeSyncWithChan
func (c *Consumer) QueueSubscribeSyncWithChan(queue string, ch chan *nats.Msg) (sub *nats.Subscription, err error) {
	if !c.IsPushMode() {
		return nil, fmt.Errorf("consumer %s > %s is not push-based", c.stream, c.name)
	}

	return c.cfg.conn.nc.QueueSubscribeSyncWithChan(c.DeliverySubject(), queue, ch)
}

// NextMsgsChan returns a channel of messages that will be closed after timeout or msgCount is reached
func NextMsgsChan(stream string, consumer string, msgCount int, opts ...RequestOption) (msgs <-chan *nats.Msg, err error) {
	ropts, err := newreqoptions(opts...)
	if err != nil {
		return nil, err
	}

	ns, err := NextSubject(stream, consumer)
	if err != nil {
		return nil, err
	}

	ctx := ropts.ctx
	var cancel func()

	if ctx == nil {
		ctx, cancel = context.WithTimeout(context.Background(), ropts.timeout)
		defer cancel()
	}

	q := make(chan *nats.Msg, msgCount)
	done := make(chan struct{})
	ib := nats.NewInbox()

	sub, err := nc.Subscribe(ib, func(m *nats.Msg) {
		q <- m
		if len(q) == cap(q) {
			done <- struct{}{}
		}
	})
	if err != nil {
		return nil, err
	}

	defer close(q)
	defer sub.Unsubscribe()

	err = nc.PublishRequest(ns, ib, []byte(strconv.Itoa(msgCount)))
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		if len(q) > 0 {
			return q, nil
		}

		return q, ctx.Err()
	case <-done:
		return q, nil
	}
}

// NextMsgs retrieves the next n messages
func NextMsgs(stream string, consumer string, msgCount int, opts ...RequestOption) (msgs []*nats.Msg, err error) {
	q, err := NextMsgsChan(stream, consumer, msgCount, opts...)
	if err != nil {
		return nil, err
	}

	for msg := range q {
		msgs = append(msgs, msg)
	}

	return msgs, nil
}

// NextMsgs retrieves the next n messages
func (c *Consumer) NextMsgs(n int, opts ...RequestOption) (m []*nats.Msg, err error) {
	if !c.IsPullMode() {
		return nil, fmt.Errorf("consumer %s > %s is not pull-based", c.stream, c.name)
	}

	return NextMsgs(c.stream, c.name, n, append(c.cfg.ropts, opts...)...)
}

// NextMsg retrieves the next message
func (c *Consumer) NextMsg(opts ...RequestOption) (m *nats.Msg, err error) {
	msgs, err := NextMsgs(c.stream, c.name, 1, append(c.cfg.ropts, opts...)...)
	if err != nil {
		return nil, err
	}

	return msgs[0], nil
}

// State returns the Consumer state
func (c *Consumer) State() (stats server.ConsumerState, err error) {
	info, err := loadConsumerInfo(c.stream, c.name, c.cfg.conn)
	if err != nil {
		return server.ConsumerState{}, err
	}

	return info.State, nil
}

// Configuration is the Consumer configuration
func (c *Consumer) Configuration() (config server.ConsumerConfig) {
	return c.cfg.ConsumerConfig
}

// Delete deletes the Consumer, after this the Consumer object should be disposed
func (c *Consumer) Delete() (err error) {
	response, err := request(fmt.Sprintf(server.JetStreamDeleteConsumerT, c.StreamName(), c.Name()), nil, c.cfg.conn)
	if err != nil {
		return err
	}

	if IsOKResponse(response) {
		return nil
	}

	return fmt.Errorf("unknown response while removing consumer %s: %q", c.Name(), response.Data)
}

func (c *Consumer) Name() string                      { return c.name }
func (c *Consumer) IsSampled() bool                   { return c.SampleFrequency() != "" }
func (c *Consumer) IsPullMode() bool                  { return c.cfg.Delivery == "" }
func (c *Consumer) IsPushMode() bool                  { return !c.IsPullMode() }
func (c *Consumer) IsDurable() bool                   { return c.cfg.Durable != "" }
func (c *Consumer) IsEphemeral() bool                 { return !c.IsDurable() }
func (c *Consumer) StreamName() string                { return c.stream }
func (c *Consumer) DeliverySubject() string           { return c.cfg.Delivery }
func (c *Consumer) DurableName() string               { return c.cfg.Durable }
func (c *Consumer) StreamSequence() uint64            { return c.cfg.StreamSeq }
func (c *Consumer) StartTime() time.Time              { return c.cfg.StartTime }
func (c *Consumer) DeliverAll() bool                  { return c.cfg.DeliverAll }
func (c *Consumer) DeliverLast() bool                 { return c.cfg.DeliverLast }
func (c *Consumer) AckPolicy() server.AckPolicy       { return c.cfg.AckPolicy }
func (c *Consumer) AckWait() time.Duration            { return c.cfg.AckWait }
func (c *Consumer) MaxDeliver() int                   { return c.cfg.MaxDeliver }
func (c *Consumer) FilterSubject() string             { return c.cfg.FilterSubject }
func (c *Consumer) ReplayPolicy() server.ReplayPolicy { return c.cfg.ReplayPolicy }
func (c *Consumer) SampleFrequency() string           { return c.cfg.SampleFrequency }
