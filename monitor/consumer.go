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
	"strconv"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

const (
	ConsumerMonitorMetaOutstandingAckCritical = "io.nats.monitor.outstanding-ack-critical"
	ConsumerMonitorMetaWaitingCritical        = "io.nats.monitor.waiting-critical"
	ConsumerMonitorMetaUnprocessedCritical    = "io.nats.monitor.unprocessed-critical"
	ConsumerMonitorMetaLastDeliveredCritical  = "io.nats.monitor.last-delivery-critical"
	ConsumerMonitorMetaLastAckCritical        = "io.nats.monitor.last-ack-critical"
	ConsumerMonitorMetaRedeliveryCritical     = "io.nats.monitor.redelivery-critical"
	ConsumerMonitorMetaPinned                 = "io.nats.monitor.pinned"
)

type ConsumerHealthCheckF func(*jsm.Consumer, *Result, ConsumerHealthCheckOptions, api.Logger)

// ConsumerHealthCheckOptions configures the consumer check
type ConsumerHealthCheckOptions struct {
	// StreamName is the stream holding the consumer
	StreamName string `json:"stream_name" yaml:"stream_name"`
	// ConsumerName is the consumer to check
	ConsumerName string `json:"consumer_name" yaml:"consumer_name"`
	// AckOutstandingCritical is the critical threshold for outstanding acks
	AckOutstandingCritical int `json:"ack_outstanding_critical" yaml:"ack_outstanding_critical"`
	// WaitingCritical is the critical threshold for waiting pulls
	WaitingCritical int `json:"waiting_critical" yaml:"waiting_critical"`
	// UnprocessedCritical is the critical threshold for messages not yet delivered by the consumer
	UnprocessedCritical int `json:"unprocessed_critical" yaml:"unprocessed_critical"`
	// LastDeliveryCritical is the critical threshold for seconds since the last delivery attempt
	LastDeliveryCritical float64 `json:"last_delivery_critical" yaml:"last_delivery_critical"`
	// LastAckCritical is the critical threshold for seconds since the lack ack
	LastAckCritical float64 `json:"last_ack_critical" yaml:"last_ack_critical"`
	// RedeliveryCritical critical threshold for number of reported redeliveries
	RedeliveryCritical int `json:"redelivery_critical" yaml:"redelivery_critical"`
	// Pinned requires consumer be priority based and all groups have pinned clients
	Pinned bool `json:"pinned" yaml:"pinned"`

	Enabled      bool                   `json:"-" yaml:"-"`
	HealthChecks []ConsumerHealthCheckF `json:"-" yaml:"-"`
}

func ExtractConsumerHealthCheckOptions(metadata map[string]string, extraChecks ...ConsumerHealthCheckF) (*ConsumerHealthCheckOptions, error) {
	opts := &ConsumerHealthCheckOptions{
		HealthChecks: extraChecks,
	}

	return populateConsumerHealthCheckOptions(metadata, opts)
}

func populateConsumerHealthCheckOptions(metadata map[string]string, opts *ConsumerHealthCheckOptions) (*ConsumerHealthCheckOptions, error) {
	var err error
	parser := []monitorMetaParser{
		{MonitorMetaEnabled, func(v string) error {
			opts.Enabled, err = strconv.ParseBool(v)
			return err
		}},
		{ConsumerMonitorMetaOutstandingAckCritical, func(v string) error {
			opts.AckOutstandingCritical, err = strconv.Atoi(v)
			return err
		}},
		{ConsumerMonitorMetaWaitingCritical, func(v string) error {
			opts.WaitingCritical, err = strconv.Atoi(v)
			return err
		}},
		{ConsumerMonitorMetaUnprocessedCritical, func(v string) error {
			opts.UnprocessedCritical, err = strconv.Atoi(v)
			return err
		}},
		{ConsumerMonitorMetaLastDeliveredCritical, func(v string) error {
			p, err := jsm.ParseDuration(v)
			if err != nil {
				return err
			}
			opts.LastDeliveryCritical = p.Seconds()
			return err
		}},
		{ConsumerMonitorMetaLastAckCritical, func(v string) error {
			p, err := jsm.ParseDuration(v)
			if err != nil {
				return err
			}
			opts.LastAckCritical = p.Seconds()
			return err
		}},
		{ConsumerMonitorMetaRedeliveryCritical, func(v string) error {
			opts.RedeliveryCritical, err = strconv.Atoi(v)
			return err
		}},
		{ConsumerMonitorMetaPinned, func(v string) error {
			opts.Pinned, err = strconv.ParseBool(v)
			return err
		}},
	}

	for _, m := range parser {
		if v, ok := metadata[m.k]; ok {
			err = m.fn(v)
			if err != nil {
				return nil, err
			}
		}
	}

	return opts, nil
}

func ConsumerInfoHealthCheck(nfo *api.ConsumerInfo, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	consumerCheckOutstandingAck(nfo, check, opts, log)
	consumerCheckWaiting(nfo, check, opts, log)
	consumerCheckUnprocessed(nfo, check, opts, log)
	consumerCheckRedelivery(nfo, check, opts, log)
	consumerCheckLastDelivery(nfo, check, opts, log)
	consumerCheckLastAck(nfo, check, opts, log)
	consumerCheckPinned(nfo, check, opts, log)
}

func ConsumerHealthCheck(server string, nopts []nats.Option, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) error {
	if opts.StreamName == "" {
		check.Critical("stream name is required")
		return nil
	}
	if opts.ConsumerName == "" {
		check.Critical("consumer name is required")
		return nil
	}

	nc, err := nats.Connect(server, nopts...)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	mgr, err := jsm.New(nc)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	consumer, err := mgr.LoadConsumer(opts.StreamName, opts.ConsumerName)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	// make sure latest info cache is set as checks accesses it directly
	nfo, err := consumer.LatestState()
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	check.Pd(&PerfDataItem{Name: "ack_pending", Value: float64(nfo.NumAckPending), Help: "The number of messages waiting to be Acknowledged", Crit: float64(opts.AckOutstandingCritical)})
	check.Pd(&PerfDataItem{Name: "pull_waiting", Value: float64(nfo.NumWaiting), Help: "The number of waiting Pull requests", Crit: float64(opts.WaitingCritical)})
	check.Pd(&PerfDataItem{Name: "pending", Value: float64(nfo.NumPending), Help: "The number of messages that have not yet been consumed", Crit: float64(opts.UnprocessedCritical)})
	check.Pd(&PerfDataItem{Name: "redelivered", Value: float64(nfo.NumRedelivered), Help: "The number of messages currently being redelivered", Crit: float64(opts.RedeliveryCritical)})
	if nfo.Delivered.Last != nil {
		check.Pd(&PerfDataItem{Name: "last_delivery", Value: time.Since(*nfo.Delivered.Last).Seconds(), Unit: "s", Help: "Seconds since the last message was delivered", Crit: opts.LastDeliveryCritical})
	}
	if nfo.AckFloor.Last != nil {
		check.Pd(&PerfDataItem{Name: "last_ack", Value: time.Since(*nfo.AckFloor.Last).Seconds(), Unit: "s", Help: "Seconds since the last message was acknowledged", Crit: opts.LastAckCritical})
	}

	ConsumerInfoHealthCheck(&nfo, check, opts, log)

	for _, hc := range opts.HealthChecks {
		hc(consumer, check, opts, log)
	}

	return nil
}

func consumerCheckPinned(nfo *api.ConsumerInfo, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	var pinned int

	for _, group := range nfo.PriorityGroups {
		if group.PinnedClientID != "" {
			pinned++
		}
	}

	if nfo.Config.PriorityPolicy == api.PriorityPinnedClient {
		check.Pd(&PerfDataItem{Name: "pinned_groups", Value: float64(pinned), Help: "The number of consumers groups with pinned clients"})
		check.Pd(&PerfDataItem{Name: "groups", Value: float64(len(nfo.Config.PriorityGroups)), Help: "The total number of consumers groups"})
	}

	if !opts.Pinned {
		return
	}

	switch {
	case len(nfo.PriorityGroups) == 0 || len(nfo.Config.PriorityGroups) == 0 || nfo.Config.PriorityPolicy != api.PriorityPinnedClient:
		check.Critical("Not pinned client priority mode")
	case pinned != len(nfo.Config.PriorityGroups):
		log.Debugf("CRITICAL: %d / %d pinned clients", pinned, len(nfo.Config.PriorityGroups))
		check.Critical("%d / %d pinned clients", pinned, len(nfo.Config.PriorityGroups))
	default:
		check.Ok("%d pinned clients", pinned)
	}
}

func consumerCheckLastAck(nfo *api.ConsumerInfo, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	switch {
	case opts.LastAckCritical <= 0:
	case nfo.AckFloor.Last == nil:
		log.Debugf("CRITICAL: No acks")
		check.Critical("No acks")
	case time.Since(*nfo.AckFloor.Last) >= secondsToDuration(opts.LastAckCritical):
		log.Debugf("CRITICAL: Last ack %v ago", time.Since(*nfo.AckFloor.Last))
		check.Critical("Last ack %v ago", time.Since(*nfo.AckFloor.Last))
	default:
		check.Ok("Last ack %v", nfo.AckFloor.Last)
	}
}

func consumerCheckLastDelivery(nfo *api.ConsumerInfo, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	switch {
	case opts.LastDeliveryCritical <= 0:
	case nfo.Delivered.Last == nil:
		log.Debugf("CRITICAL: No deliveries")
		check.Critical("No deliveries")
	case time.Since(*nfo.Delivered.Last) >= secondsToDuration(opts.LastDeliveryCritical):
		log.Debugf("CRITICAL: Last delivery %v", nfo.Delivered.Last.Format(time.DateTime))
		check.Critical("Last delivery %s ago", time.Since(*nfo.Delivered.Last))
	default:
		check.Ok("Last delivery %v", nfo.Delivered.Last)
	}
}

func consumerCheckRedelivery(nfo *api.ConsumerInfo, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	switch {
	case opts.RedeliveryCritical <= 0:
		return
	case nfo.NumRedelivered >= opts.RedeliveryCritical:
		log.Debugf("CRITICAL Redelivered: %v", nfo.NumRedelivered)
		check.Critical("Redelivered: %v", nfo.NumRedelivered)
	default:
		check.Ok("Redelivered: %v", nfo.NumRedelivered)
	}
}

func consumerCheckUnprocessed(nfo *api.ConsumerInfo, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	switch {
	case opts.UnprocessedCritical <= 0:
		return
	case nfo.NumPending >= uint64(opts.UnprocessedCritical):
		log.Debugf("CRITICAL Unprocessed Messages: %v", nfo.NumAckPending)
		check.Critical("Unprocessed Messages: %v", nfo.NumAckPending)
	default:
		check.Ok("Unprocessed Messages: %v", nfo.NumAckPending)
	}
}

func consumerCheckWaiting(nfo *api.ConsumerInfo, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	switch {
	case opts.WaitingCritical <= 0:
		return
	case nfo.NumWaiting >= opts.WaitingCritical:
		log.Debugf("CRITICAL Waiting Pulls: %v", nfo.NumWaiting)
		check.Critical("Waiting Pulls: %v", nfo.NumWaiting)
	default:
		check.Ok("Waiting Pulls: %v", nfo.NumWaiting)
	}
}

func consumerCheckOutstandingAck(nfo *api.ConsumerInfo, check *Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	switch {
	case opts.AckOutstandingCritical <= 0:
		return
	case nfo.NumAckPending >= opts.AckOutstandingCritical:
		log.Debugf("CRITICAL Ack Pending: %v", nfo.NumAckPending)
		check.Critical("Ack Pending: %v", nfo.NumAckPending)
	default:
		check.Ok("Ack Pending: %v", nfo.NumAckPending)
	}
}
