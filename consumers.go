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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

// DefaultConsumer is the configuration that will be used to create new Consumers in NewConsumer
var DefaultConsumer = api.ConsumerConfig{
	DeliverPolicy: api.DeliverAll,
	AckPolicy:     api.AckExplicit,
	AckWait:       30 * time.Second,
	ReplayPolicy:  api.ReplayInstant,
}

// SampledDefaultConsumer is the configuration that will be used to create new Consumers in NewConsumer
var SampledDefaultConsumer = api.ConsumerConfig{
	DeliverPolicy:   api.DeliverAll,
	AckPolicy:       api.AckExplicit,
	AckWait:         30 * time.Second,
	ReplayPolicy:    api.ReplayInstant,
	SampleFrequency: "100%",
}

// ConsumerOptions configures consumers
type ConsumerOption func(o *ConsumerCfg) error

// Consumer represents a JetStream consumer
type Consumer struct {
	name   string
	stream string
	cfg    *ConsumerCfg
}

type ConsumerCfg struct {
	api.ConsumerConfig

	conn  *reqoptions
	ropts []RequestOption
}

// NewConsumerFromDefault creates a new consumer based on a template config that gets modified by opts
func NewConsumerFromDefault(stream string, dflt api.ConsumerConfig, opts ...ConsumerOption) (consumer *Consumer, err error) {
	cfg, err := NewConsumerConfiguration(dflt, opts...)
	if err != nil {
		return nil, err
	}

	valid, errs := cfg.Validate()
	if !valid {
		return nil, fmt.Errorf("configuration validation failed: %s", strings.Join(errs, ", "))
	}

	req := api.CreateConsumerRequest{
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

func createDurableConsumer(req api.CreateConsumerRequest, opts *reqoptions) (name string, err error) {
	var resp api.JetStreamCreateConsumerResponse
	err = jsonRequest(fmt.Sprintf(api.JetStreamCreateConsumerT, req.Stream, req.Config.Durable), req, &resp, opts)
	if err != nil {
		return "", err
	}

	return resp.Name, nil
}

func createEphemeralConsumer(req api.CreateConsumerRequest, opts *reqoptions) (name string, err error) {
	var resp api.JetStreamCreateConsumerResponse
	err = jsonRequest(fmt.Sprintf(api.JetStreamCreateEphemeralConsumerT, req.Stream), req, &resp, opts)
	if err != nil {
		return "", err
	}

	return resp.Name, nil
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
func LoadOrNewConsumerFromDefault(stream string, name string, template api.ConsumerConfig, opts ...ConsumerOption) (consumer *Consumer, err error) {
	cfg, err := NewConsumerConfiguration(template, opts...)
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
		cfg: &ConsumerCfg{
			ConsumerConfig: api.ConsumerConfig{},
			conn:           conn,
			ropts:          opts,
		},
	}

	err = loadConfigForConsumer(consumer, conn)
	if err != nil {
		return nil, err
	}

	return consumer, nil
}

// NewConsumerConfiguration generates a new configuration based on template modified by opts
func NewConsumerConfiguration(dflt api.ConsumerConfig, opts ...ConsumerOption) (*ConsumerCfg, error) {
	cfg := &ConsumerCfg{
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

func loadConfigForConsumer(consumer *Consumer, opts *reqoptions) (err error) {
	info, err := loadConsumerInfo(consumer.stream, consumer.name, opts)
	if err != nil {
		return err
	}

	consumer.cfg.ConsumerConfig = info.Config

	return nil
}

func loadConsumerInfo(s string, c string, opts *reqoptions) (info api.ConsumerInfo, err error) {
	var resp api.JetStreamConsumerInfoResponse

	err = jsonRequest(fmt.Sprintf(api.JetStreamConsumerInfoT, s, c), nil, &resp, opts)
	if err != nil {
		return info, err
	}

	return *resp.ConsumerInfo, nil
}

func DeliverySubject(s string) ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.DeliverSubject = s
		return nil
	}
}

func DurableName(s string) ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.Durable = s
		return nil
	}
}

func StartAtSequence(s uint64) ConsumerOption {
	return func(o *ConsumerCfg) error {
		resetDeliverPolicy(o)
		o.ConsumerConfig.DeliverPolicy = api.DeliverByStartSequence
		o.ConsumerConfig.OptStartSeq = s
		return nil
	}
}

func StartAtTime(t time.Time) ConsumerOption {
	return func(o *ConsumerCfg) error {
		resetDeliverPolicy(o)
		o.ConsumerConfig.DeliverPolicy = api.DeliverByStartTime
		o.ConsumerConfig.OptStartTime = &t
		return nil
	}
}

func DeliverAllAvailable() ConsumerOption {
	return func(o *ConsumerCfg) error {
		resetDeliverPolicy(o)
		o.ConsumerConfig.DeliverPolicy = api.DeliverAll
		return nil
	}
}

func StartWithLastReceived() ConsumerOption {
	return func(o *ConsumerCfg) error {
		resetDeliverPolicy(o)
		o.ConsumerConfig.DeliverPolicy = api.DeliverLast
		return nil
	}
}

func StartWithNextReceived() ConsumerOption {
	return func(o *ConsumerCfg) error {
		resetDeliverPolicy(o)
		o.ConsumerConfig.DeliverPolicy = api.DeliverNew
		return nil
	}
}

func StartAtTimeDelta(d time.Duration) ConsumerOption {
	return func(o *ConsumerCfg) error {
		resetDeliverPolicy(o)

		t := time.Now().Add(-1 * d)
		o.ConsumerConfig.DeliverPolicy = api.DeliverByStartTime
		o.ConsumerConfig.OptStartTime = &t
		return nil
	}
}

func resetDeliverPolicy(o *ConsumerCfg) {
	o.ConsumerConfig.DeliverPolicy = api.DeliverAll
	o.ConsumerConfig.OptStartSeq = 0
	o.ConsumerConfig.OptStartTime = nil
}

func AcknowledgeNone() ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.AckPolicy = api.AckNone
		return nil
	}
}

func AcknowledgeAll() ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.AckPolicy = api.AckAll
		return nil
	}
}

func AcknowledgeExplicit() ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.AckPolicy = api.AckExplicit
		return nil
	}
}

func AckWait(t time.Duration) ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.AckWait = t
		return nil
	}
}

func MaxDeliveryAttempts(n int) ConsumerOption {
	return func(o *ConsumerCfg) error {
		if n == 0 {
			return fmt.Errorf("configuration would prevent all deliveries")
		}
		o.ConsumerConfig.MaxDeliver = n
		return nil
	}
}

func FilterStreamBySubject(s string) ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.FilterSubject = s
		return nil
	}
}

func ReplayInstantly() ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.ReplayPolicy = api.ReplayInstant
		return nil
	}
}

func ReplayAsReceived() ConsumerOption {
	return func(o *ConsumerCfg) error {
		o.ConsumerConfig.ReplayPolicy = api.ReplayOriginal
		return nil
	}
}

func SamplePercent(i int) ConsumerOption {
	return func(o *ConsumerCfg) error {
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
	return func(o *ConsumerCfg) error {
		for _, opt := range opts {
			opt(o.conn)
		}

		o.ropts = append(o.ropts, opts...)

		return nil
	}
}

// Reset reloads the Consumer configuration from the JetStream server
func (c *Consumer) Reset(opts ...RequestOption) error {
	ropts, err := newreqoptions(opts...)
	if err != nil {
		if err != nil {
			return err
		}
	}

	return loadConfigForConsumer(c, ropts)
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

	return fmt.Sprintf(api.JetStreamRequestNextT, stream, consumer), nil
}

// AckSampleSubject is the subject used to publish ack samples to
func (c *Consumer) AckSampleSubject() string {
	if c.SampleFrequency() == "" {
		return ""
	}

	return api.JetStreamMetricConsumerAckPre + "." + c.StreamName() + "." + c.name
}

// AdvisorySubject is a wildcard subscription subject that subscribes to all advisories for this consumer
func (c *Consumer) AdvisorySubject() string {
	return api.JetStreamAdvisoryPrefix + "." + "*" + "." + c.StreamName() + "." + c.name
}

// MetricSubject is a wildcard subscription subject that subscribes to all metrics for this consumer
func (c *Consumer) MetricSubject() string {
	return api.JetStreamMetricPrefix + "." + "*" + "." + c.StreamName() + "." + c.name
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

func NextMsg(stream string, consumer string, opts ...RequestOption) (msgs *nats.Msg, err error) {
	ropts, err := newreqoptions(opts...)
	if err != nil {
		if err != nil {
			return nil, err
		}
	}

	s, err := NextSubject(stream, consumer)
	if err != nil {
		return nil, err
	}

	return request(s, []byte(strconv.Itoa(1)), ropts)
}

// NextMsg retrieves the next message
func (c *Consumer) NextMsg(opts ...RequestOption) (m *nats.Msg, err error) {
	return NextMsg(c.stream, c.name, append(c.cfg.ropts, opts...)...)
}

// DeliveredState reports the messages sequences that were successfully delivered
func (c *Consumer) DeliveredState(opts ...RequestOption) (stats api.SequencePair, err error) {
	info, err := c.State(opts...)
	if err != nil {
		return api.SequencePair{}, err
	}

	return info.Delivered, nil
}

// AcknowledgedFloor reports the highest contiguous message sequences that were acknowledged
func (c *Consumer) AcknowledgedFloor(opts ...RequestOption) (stats api.SequencePair, err error) {
	info, err := c.State(opts...)
	if err != nil {
		return api.SequencePair{}, err
	}

	return info.AckFloor, nil
}

// PendingMessageCount reports the number of messages sent but not yet acknowledged
func (c *Consumer) PendingMessageCount(opts ...RequestOption) (int, error) {
	info, err := c.State(opts...)
	if err != nil {
		return 0, err
	}

	return info.NumPending, nil
}

// RedeliveryCount reports the number of redelivers that were done
func (c *Consumer) RedeliveryCount(opts ...RequestOption) (int, error) {
	info, err := c.State(opts...)
	if err != nil {
		return 0, err
	}

	return info.NumRedelivered, nil
}

// State loads a snapshot of consumer state including delivery counts, retries and more
func (c *Consumer) State(opts ...RequestOption) (api.ConsumerInfo, error) {
	ropts, err := newreqoptions(append(c.cfg.ropts, opts...)...)
	if err != nil {
		if err != nil {
			return api.ConsumerInfo{}, err
		}
	}

	return loadConsumerInfo(c.stream, c.name, ropts)
}

// Configuration is the Consumer configuration
func (c *Consumer) Configuration() (config api.ConsumerConfig) {
	return c.cfg.ConsumerConfig
}

// Delete deletes the Consumer, after this the Consumer object should be disposed
func (c *Consumer) Delete() (err error) {
	var resp api.JetStreamDeleteConsumerResponse
	err = jsonRequest(fmt.Sprintf(api.JetStreamDeleteConsumerT, c.StreamName(), c.Name()), nil, &resp, c.cfg.conn)
	if err != nil {
		return err
	}

	if resp.Success {
		return nil
	}

	return fmt.Errorf("unknown response while removing consumer %s", c.Name())
}

func (c *Consumer) Name() string                     { return c.name }
func (c *Consumer) IsSampled() bool                  { return c.SampleFrequency() != "" }
func (c *Consumer) IsPullMode() bool                 { return c.cfg.DeliverSubject == "" }
func (c *Consumer) IsPushMode() bool                 { return !c.IsPullMode() }
func (c *Consumer) IsDurable() bool                  { return c.cfg.Durable != "" }
func (c *Consumer) IsEphemeral() bool                { return !c.IsDurable() }
func (c *Consumer) StreamName() string               { return c.stream }
func (c *Consumer) DeliverySubject() string          { return c.cfg.DeliverSubject }
func (c *Consumer) DurableName() string              { return c.cfg.Durable }
func (c *Consumer) StartSequence() uint64            { return c.cfg.OptStartSeq }
func (c *Consumer) DeliverPolicy() api.DeliverPolicy { return c.cfg.DeliverPolicy }
func (c *Consumer) AckPolicy() api.AckPolicy         { return c.cfg.AckPolicy }
func (c *Consumer) AckWait() time.Duration           { return c.cfg.AckWait }
func (c *Consumer) MaxDeliver() int                  { return c.cfg.MaxDeliver }
func (c *Consumer) FilterSubject() string            { return c.cfg.FilterSubject }
func (c *Consumer) ReplayPolicy() api.ReplayPolicy   { return c.cfg.ReplayPolicy }
func (c *Consumer) SampleFrequency() string          { return c.cfg.SampleFrequency }
func (c *Consumer) StartTime() time.Time {
	if c.cfg.OptStartTime == nil {
		return time.Time{}
	}
	return *c.cfg.OptStartTime
}
