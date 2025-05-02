package jsm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

type StreamPager struct {
	mgr      *Manager
	sub      *nats.Subscription
	consumer *Consumer

	q             chan *nats.Msg
	stream        *Stream
	startSeq      int
	startDelta    time.Duration
	pageSize      int
	filterSubject string
	started       bool
	timeout       time.Duration
	seen          int

	useDirect bool
	curSeq    uint64 // for direct where to get

	mu sync.Mutex
}

// PagerOption configures the stream pager
type PagerOption func(p *StreamPager)

// PagerStartId sets a starting stream sequence for the pager
func PagerStartId(id int) PagerOption {
	return func(p *StreamPager) {
		p.startSeq = id
	}
}

// PagerFilterSubject sets a filter subject for the pager
func PagerFilterSubject(s string) PagerOption {
	return func(p *StreamPager) {
		p.filterSubject = s
	}
}

// PagerStartDelta sets a starting time delta for the pager
func PagerStartDelta(d time.Duration) PagerOption {
	return func(p *StreamPager) {
		p.startDelta = d
	}
}

// PagerSize is the size of pages to walk
func PagerSize(sz int) PagerOption {
	return func(p *StreamPager) {
		p.pageSize = sz
	}
}

// PagerTimeout is the time to wait for messages before it's assumed the end of the stream was reached
func PagerTimeout(d time.Duration) PagerOption {
	return func(p *StreamPager) {
		p.timeout = d
	}
}

func (p *StreamPager) start(stream *Stream, mgr *Manager, opts ...PagerOption) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return fmt.Errorf("already started")
	}

	p.stream = stream
	p.mgr = mgr
	p.startDelta = 0
	p.startSeq = -1
	p.seen = -1
	p.filterSubject = ">"

	for _, o := range opts {
		o(p)
	}

	if p.timeout == 0 {
		p.timeout = 5 * time.Second
	}

	if p.pageSize == 0 {
		p.pageSize = 25
	}

	var err error

	if !p.stream.DirectAllowed() {
		if p.stream.Retention() == api.WorkQueuePolicy {
			return fmt.Errorf("work queue retention streams can only be paged if direct access is allowed")
		}

		if p.stream.Retention() == api.InterestPolicy {
			return fmt.Errorf("interest retention streams can only be paged if direct access is allowed")
		}
	}

	// for now only on WQ because its slow, until there is a batch mode direct request
	p.useDirect = p.stream.Retention() == api.WorkQueuePolicy || p.stream.Retention() == api.InterestPolicy

	p.q = make(chan *nats.Msg, p.pageSize)
	p.sub, err = mgr.nc.ChanSubscribe(mgr.nc.NewRespInbox(), p.q)
	if err != nil {
		p.close()
		return err
	}

	if p.useDirect {
		if p.startSeq > 0 {
			p.curSeq = uint64(p.startSeq)
		} else {
			nfo, err := p.stream.State()
			if err != nil {
				return err
			}
			p.curSeq = nfo.FirstSeq
		}

		if p.startDelta > 0 {
			return fmt.Errorf("workqueue paging does not support time delta starting positions")
		}
	} else {
		err = p.createConsumer()
		if err != nil {
			p.close()
			return err
		}
	}

	p.started = true

	return nil
}

func (p *StreamPager) directGetBatch() error {
	// the idea here is to fetch a batch matching page to fill up the queue but direct makes this difficult
	// because different stream replicas can answer and also the message that causes 404 replies much faster than
	// one that includes data so it ends up out of order and just weird
	//
	// so until the batch api launch we have to do a get on demand to preserve order and to handle the 404 correctly

	req := api.JSApiMsgGetRequest{Seq: p.curSeq, NextFor: p.filterSubject}

	rj, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if p.mgr.trace {
		log.Printf(">>> %s\n%s\n\n", p.stream.DirectSubject(), string(rj))
	}

	err = p.mgr.nc.PublishRequest(p.stream.DirectSubject(), p.sub.Subject, rj)
	if err != nil {
		return err
	}

	p.curSeq++

	return nil
}

func (p *StreamPager) fetchBatch() error {
	req := api.JSApiConsumerGetNextRequest{Batch: p.pageSize, NoWait: true}
	rj, err := json.Marshal(req)
	if err != nil {
		return err
	}

	err = p.mgr.nc.PublishRequest(p.consumer.NextSubject(), p.sub.Subject, rj)
	if err != nil {
		return err
	}

	return nil
}

// NextMsg retrieves the next message from the pager interrupted by ctx.
//
// last indicates if the message is the last in the current page, the next call
// to NextMsg will first request the next page, if the client is prompting users
// to continue to the next page it should be done when last is true
//
// When the end of the stream is reached err will be non nil and last will be true
// otherwise err being non nil while last is false indicate a failed state. End is indicated
// by no new messages arriving after ctx timeout or the time set using PagerTimes() is reached
func (p *StreamPager) NextMsg(ctx context.Context) (msg *nats.Msg, last bool, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.useDirect {
		err = p.directGetBatch()
		if err != nil {
			return nil, false, err
		}
	}

	if p.seen == p.pageSize || p.seen == -1 {
		p.seen = 0

		if !p.useDirect {
			err = p.fetchBatch()
			if err != nil {
				return nil, false, err
			}
		}
	}

	timeout, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	select {
	case msg := <-p.q:
		if p.mgr.trace {
			log.Printf("<<< (%d) %s\n%v\n", len(msg.Data), msg.Header, string(msg.Data))
		}

		p.seen++

		status := msg.Header.Get("Status")
		if status == "404" || status == "408" {
			return nil, true, fmt.Errorf("last message reached")
		}

		if p.useDirect {
			msg.Subject = msg.Header.Get("Nats-Subject")
		} else {
			msg.Ack()
		}

		return msg, p.seen == p.pageSize, nil

	case <-timeout.Done():
		return nil, true, fmt.Errorf("timeout waiting for new messages")
	}
}

func (p *StreamPager) createConsumer() error {
	cops := []ConsumerOption{
		ConsumerDescription("JSM Stream Pager"),
		InactiveThreshold(time.Hour),
		DurableName(fmt.Sprintf("stream_pager_%d%d", os.Getpid(), time.Now().UnixNano())),
		ConsumerOverrideMemoryStorage(),
		ConsumerOverrideReplicas(1),
	}

	switch {
	case p.startDelta > 0:
		cops = append(cops, StartAtTimeDelta(p.startDelta))
	case p.startSeq > -1:
		cops = append(cops, StartAtSequence(uint64(p.startSeq)))
	case p.startDelta == 0 && p.startSeq == -1:
		cops = append(cops, DeliverAllAvailable())
	default:
		return fmt.Errorf("no valid start options specified")
	}

	if p.filterSubject != "" {
		cops = append(cops, FilterStreamBySubject(p.filterSubject))
	}

	var err error
	p.consumer, err = p.mgr.NewConsumer(p.stream.Name(), cops...)
	if err != nil {
		return err
	}

	if p.useDirect {
		nfo, err := p.consumer.LatestState()
		if err != nil {
			return err
		}

		// record the initial seq based on consumer config like time deltas etc
		// direct will start from there following subject filter
		p.curSeq = nfo.Delivered.Stream
	}

	return nil
}

func (p *StreamPager) close() error {
	if p.sub != nil {
		p.sub.Unsubscribe()
	}

	if p.consumer != nil {
		err := p.consumer.Delete()
		if err != nil {
			return err
		}
	}

	close(p.q)

	p.started = false

	return nil
}

// Close dispose of the resources used by the pager and should be called when done
func (p *StreamPager) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.close()
}
