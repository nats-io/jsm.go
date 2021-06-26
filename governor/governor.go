package governor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

type Finisher func() error

type Governor interface {
	// Start attempts to get a spot in the Governor, gives up on context, call Finisher to signal end of work
	Start(ctx context.Context, name string) (Finisher, error)
}

// Manager controls concurrent executions of work distributed throughout a nats network by using
// a stream as a capped stack where workers reserve a slot and later release the slot
type Manager interface {
	Limit() int64
	SetLimit(uint64) error
	MaxAge() time.Duration
	SetMaxAge(time.Duration) error
	Name() string
	Stream() *jsm.Stream
}

type jsGMgr struct {
	name     string
	stream   string
	maxAge   time.Duration
	limit    uint64
	mgr      *jsm.Manager
	nc       *nats.Conn
	str      *jsm.Stream
	subj     string
	replicas int
	running  bool

	mu sync.Mutex
}

func NewJSGovernorManager(name string, limit uint64, maxAge time.Duration, replicas uint, mgr *jsm.Manager, update bool) (Manager, error) {
	gov := &jsGMgr{
		name:     name,
		stream:   fmt.Sprintf("GOVERNOR_%s", name),
		subj:     fmt.Sprintf("$GOVERNOR.%s", name),
		maxAge:   maxAge,
		limit:    limit,
		mgr:      mgr,
		nc:       mgr.NatsConn(),
		replicas: int(replicas),
	}

	err := gov.loadOrCreate(update)
	if err != nil {
		return nil, err
	}

	return gov, nil
}

func NewJSGovernor(name string, mgr *jsm.Manager) Governor {
	gov := &jsGMgr{
		name:   name,
		stream: fmt.Sprintf("GOVERNOR_%s", name),
		subj:   fmt.Sprintf("$GOVERNOR.%s", name),
		mgr:    mgr,
		nc:     mgr.NatsConn(),
	}

	return gov
}

func (g *jsGMgr) Start(ctx context.Context, name string) (Finisher, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.running = true
	seq := uint64(0)

	try := func() error {
		m, err := g.nc.RequestWithContext(ctx, g.subj, []byte(name))
		if err != nil {
			return err
		}

		res, err := jsm.ParsePubAck(m)
		if err != nil {
			return err
		}

		seq = res.Sequence

		return nil
	}

	closer := func() error {
		if seq == 0 {
			return nil
		}

		err := g.mgr.DeleteStreamMessage(g.stream, seq, true)
		if err != nil {
			return fmt.Errorf("could not remove seq %d: %s", seq, err)
		}

		return nil
	}

	err := try()
	if err == nil {
		return closer, nil
	}

	ticker := time.NewTicker(50 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			err = try()
			if err == nil {
				return closer, nil
			}

		case <-ctx.Done():
			ticker.Stop()
			return nil, ctx.Err()
		}
	}
}

func (g *jsGMgr) Stream() *jsm.Stream   { return g.str }
func (g *jsGMgr) Limit() int64          { return g.str.MaxMsgs() }
func (g *jsGMgr) MaxAge() time.Duration { return g.str.MaxAge() }
func (g *jsGMgr) Name() string          { return g.name }
func (g *jsGMgr) SetLimit(limit uint64) error {
	g.mu.Lock()
	g.limit = limit
	g.mu.Unlock()

	return g.updateConfig()
}

func (g *jsGMgr) SetMaxAge(age time.Duration) error {
	g.mu.Lock()
	g.maxAge = age
	g.mu.Unlock()

	return g.updateConfig()
}

func (g *jsGMgr) updateConfig() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.str.MaxAge() != g.maxAge || g.str.MaxMsgs() != int64(g.limit) {
		err := g.str.UpdateConfiguration(g.str.Configuration(), g.streamOpts()...)
		if err != nil {
			return fmt.Errorf("stream update failed: %s", err)
		}
	}

	return nil
}

func (g *jsGMgr) streamOpts() []jsm.StreamOption {
	opts := []jsm.StreamOption{
		jsm.MaxAge(g.maxAge),
		jsm.MaxMessages(int64(g.limit)),
		jsm.Subjects(g.subj),
		jsm.Replicas(g.replicas),
		jsm.LimitsRetention(),
		jsm.FileStorage(),
		jsm.DiscardNew(),
	}

	if g.replicas > 0 {
		opts = append(opts, jsm.Replicas(g.replicas))
	}

	return opts
}

func (g *jsGMgr) loadOrCreate(update bool) error {
	opts := g.streamOpts()

	str, err := g.mgr.LoadOrNewStream(g.stream, opts...)
	if err != nil {
		return err
	}

	g.str = str

	if update {
		g.updateConfig()
	}

	return nil
}
