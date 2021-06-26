package governor

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nats-io/jsm.go"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startJSServer(t *testing.T) (*natsd.Server, *nats.Conn, *jsm.Manager) {
	t.Helper()

	d, err := ioutil.TempDir("", "jstest")
	if err != nil {
		t.Fatalf("temp dir could not be made: %s", err)
	}

	opts := &natsd.Options{
		ServerName: "test.example.net",
		JetStream:  true,
		StoreDir:   d,
		Port:       -1,
		Host:       "localhost",
		// LogFile:    "/tmp/server.log",
		// Trace:        true,
		// TraceVerbose: true,
		Cluster: natsd.ClusterOpts{Name: "gotest"},
	}

	s, err := natsd.NewServer(opts)
	if err != nil {
		t.Fatalf("server start failed: %s", err)
	}

	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		t.Fatalf("nats server did not start")
	}

	// s.ConfigureLogger()

	nc, err := nats.Connect(s.ClientURL(), nats.UseOldRequestStyle(), nats.MaxReconnects(-1))
	if err != nil {
		t.Fatalf("client start failed: %s", err)
	}

	mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
	if err != nil {
		t.Fatalf("manager creation failed: %s", err)
	}

	return s, nc, mgr
}

func TestJsGovernor(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	limit := 100

	gmgr, err := NewJSGovernorManager("TEST", uint64(limit), 2*time.Minute, 0, mgr, true)
	if err != nil {
		t.Fatalf("manager failed: %s", err)
	}

	if gmgr.Name() != "TEST" {
		t.Fatalf("Expected TEST got %v", gmgr.Name())
	}

	if gmgr.Limit() != int64(limit) {
		t.Fatalf("Expected limit of %d got %d", limit, gmgr.Limit())
	}

	if gmgr.MaxAge() != 2*time.Minute {
		t.Fatalf("Expected max age 2 minutes got %d", gmgr.MaxAge())
	}

	if gmgr.Stream().Name() != "GOVERNOR_TEST" {
		t.Fatalf("Stream had wrong name: %s", gmgr.Stream().Name())
	}

	if !cmp.Equal(gmgr.Stream().Subjects(), []string{"$GOVERNOR.campaign.TEST"}) {
		t.Fatalf("Stream had wrong subjects: %v", gmgr.Stream().Subjects())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	workers := 1000
	max := 0
	current := 0
	cnt := 0
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	var errs []string

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(t *testing.T, i int) {
			defer wg.Done()

			g := NewJSGovernor("TEST", mgr, WithInterval(10*time.Millisecond))

			name := fmt.Sprintf("worker %d", i)
			finisher, err := g.Start(ctx, name)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%d did not start: %s", i, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			cnt++
			current++
			if max < current {
				max = current
			}
			mu.Unlock()

			// give the scheduler a chance
			time.Sleep(50 * time.Millisecond)

			// before finish because its very quick and another one starts before this happens if its after finished call
			mu.Lock()
			current--
			mu.Unlock()

			err = finisher()
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%d finished failed: %s", i, err))
				mu.Unlock()
				return
			}
		}(t, i)
	}

	for {
		if ctx.Err() != nil {
			t.Fatalf("timeout %s", ctx.Err())
		}

		mu.Lock()
		if cnt == workers {
			if max > limit {
				t.Fatalf("had more than %d concurrent: %d", limit, max)
			}
			mu.Unlock()

			wg.Wait()

			if len(errs) > 0 {
				t.Fatalf("Had errors in workers: %s", strings.Join(errs, ", "))
			}

			log.Printf("ran: %d, max concurrent: %d", cnt, max)

			return
		}
		mu.Unlock()

		time.Sleep(time.Millisecond)
	}
}
