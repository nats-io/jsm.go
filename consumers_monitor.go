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

package jsm

import (
	"strconv"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/monitor"
)

const (
	ConsumerMonitorMetaOutstandingAckCritical = "io.nats.monitor.outstanding-ack-critical"
	ConsumerMonitorMetaWaitingCritical        = "io.nats.monitor.waiting-critical"
	ConsumerMonitorMetaUnprocessedCritical    = "io.nats.monitor.unprocessed-critical"
	ConsumerMonitorMetaLastDeliveredCritical  = "io.nats.monitor.last-delivery-critical"
	ConsumerMonitorMetaLastAckCritical        = "io.nats.monitor.last-ack-critical"
	ConsumerMonitorMetaRedeliveryCritical     = "io.nats.monitor.redelivery-critical"
)

type ConsumerHealthCheck func(*Consumer, *monitor.Result, ConsumerHealthCheckOptions, api.Logger)

type ConsumerHealthCheckOptions struct {
	Enabled                bool
	AckOutstandingCritical int
	WaitingCritical        int
	UnprocessedCritical    int
	LastDeliveryCritical   time.Duration
	LastAckCritical        time.Duration
	RedeliveryCritical     int
	HealthChecks           []ConsumerHealthCheck
}

func (c *Consumer) MonitorOptions(extraChecks ...ConsumerHealthCheck) (*ConsumerHealthCheckOptions, error) {
	opts := &ConsumerHealthCheckOptions{
		HealthChecks: extraChecks,
	}

	var err error
	parser := []monitorMetaParser{
		{MonitorEnabled, func(v string) error {
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
			opts.LastDeliveryCritical, err = parseDuration(v)
			return err
		}},
		{ConsumerMonitorMetaLastAckCritical, func(v string) error {
			opts.LastAckCritical, err = parseDuration(v)
			return err
		}},
		{ConsumerMonitorMetaRedeliveryCritical, func(v string) error {
			opts.RedeliveryCritical, err = strconv.Atoi(v)
			return err
		}},
	}

	metadata := c.Metadata()

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

func (c *Consumer) HealthCheck(opts ConsumerHealthCheckOptions, check *monitor.Result, log api.Logger) (*monitor.Result, error) {
	if check == nil {
		check = &monitor.Result{
			Check: "consumer_status",
			Name:  c.Name(),
		}
	}

	// make sure latest info cache is set as checks accesses it directly
	nfo, err := c.LatestState()
	if err != nil {
		return nil, err
	}

	c.checkOutstandingAck(&nfo, check, opts, log)
	c.checkWaiting(&nfo, check, opts, log)
	c.checkUnprocessed(&nfo, check, opts, log)
	c.checkRedelivery(&nfo, check, opts, log)
	c.checkLastDelivery(&nfo, check, opts, log)
	c.checkLastAck(&nfo, check, opts, log)

	for _, hc := range opts.HealthChecks {
		hc(c, check, opts, log)
	}

	return check, nil
}

func (c *Consumer) checkLastAck(nfo *api.ConsumerInfo, check *monitor.Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	switch {
	case opts.LastAckCritical <= 0:
	case nfo.AckFloor.Last == nil:
		log.Debugf("CRITICAL: No acks")
		check.Critical("No acks")
	case time.Since(*nfo.AckFloor.Last) >= opts.LastAckCritical:
		log.Debugf("CRITICAL: Last ack %v ago", time.Since(*nfo.AckFloor.Last))
		check.Critical("Last ack %v ago", time.Since(*nfo.AckFloor.Last))
	default:
		check.Ok("Last ack %v", nfo.AckFloor.Last)
	}
}

func (c *Consumer) checkLastDelivery(nfo *api.ConsumerInfo, check *monitor.Result, opts ConsumerHealthCheckOptions, log api.Logger) {
	switch {
	case opts.LastDeliveryCritical <= 0:
	case nfo.Delivered.Last == nil:
		log.Debugf("CRITICAL: No deliveries")
		check.Critical("No deliveries")
	case time.Since(*nfo.Delivered.Last) >= opts.LastDeliveryCritical:
		log.Debugf("CRITICAL: Last delivery %v", nfo.Delivered.Last.Format(time.DateTime))
		check.Critical("Last delivery %s ago", time.Since(*nfo.Delivered.Last))
	default:
		check.Ok("Last delivery %v", nfo.Delivered.Last)
	}
}

func (c *Consumer) checkRedelivery(nfo *api.ConsumerInfo, check *monitor.Result, opts ConsumerHealthCheckOptions, log api.Logger) {
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

func (c *Consumer) checkUnprocessed(nfo *api.ConsumerInfo, check *monitor.Result, opts ConsumerHealthCheckOptions, log api.Logger) {
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

func (c *Consumer) checkWaiting(nfo *api.ConsumerInfo, check *monitor.Result, opts ConsumerHealthCheckOptions, log api.Logger) {
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

func (c *Consumer) checkOutstandingAck(nfo *api.ConsumerInfo, check *monitor.Result, opts ConsumerHealthCheckOptions, log api.Logger) {
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
