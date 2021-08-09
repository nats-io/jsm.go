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

package election

import (
	"context"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestElection(t *testing.T) {
	cases := []struct {
		kind string
		opt  jsm.StreamOption
	}{
		{"memory", jsm.MemoryStorage()},
		{"disk", jsm.FileStorage()},
	}

	for _, store := range cases {
		t.Run(store.kind, func(t *testing.T) {
			srv, nc, mgr := startJSServer(t)
			defer srv.Shutdown()
			defer nc.Close()

			stream, err := mgr.NewStream("ELECTION", store.opt, jsm.MaxConsumers(1), jsm.MaxMessages(1), jsm.DiscardOld(), jsm.LimitsRetention())
			if err != nil {
				t.Fatalf("stream create failed:: %s", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			wg := sync.WaitGroup{}
			mu := sync.Mutex{}
			wins := 0
			lost := 0
			active := 0
			maxActive := 0

			worker := func(wg *sync.WaitGroup, i int) {
				defer wg.Done()

				winCb := func() {
					mu.Lock()
					wins++
					active++
					if active > maxActive {
						maxActive = active
					}
					act := active
					mu.Unlock()
					log.Printf("%d became leader with %d active leaders", i, act)
				}

				lostCb := func() {
					log.Printf("%d lost leadership", i)
					mu.Lock()
					lost++
					active--
					mu.Unlock()
				}

				nc, err := nats.Connect(srv.ClientURL(), nats.MaxReconnects(-1))
				if err != nil {
					t.Fatalf("nc failed: %s", err)
				}
				mgr, _ := jsm.New(nc)

				elect, err := NewElection("TEST_"+strconv.Itoa(i), winCb, lostCb, "ELECTION", mgr)
				if err != nil {
					t.Fatalf("election start failed: %s", err)
				}
				elect.Start(ctx)
			}

			for i := 0; i < 10; i++ {
				wg.Add(1)
				go worker(&wg, i)
			}

			kills := 0
			// kills consumers off to simulate errors
			for {
				consumers, err := stream.ConsumerNames()
				if err != nil {
					t.Fatalf("could not get names: %s", err)
				}

				for _, c := range consumers {
					consumer, _ := stream.LoadConsumer(c)
					if consumer != nil {
						log.Printf("simulating failure of consumer %s", consumer.Description())
						consumer.Delete()
						kills++
					}
				}

				if kills > 1 && kills%2 == 0 {
					log.Printf("deleting stream %s", stream.Name())
					err = stream.Delete()
					if err != nil {
						t.Fatalf("stream delete failed: %s", err)
					}
				}

				err = ctxSleep(ctx, 2*time.Second)
				if err != nil {
					break
				}
			}

			wg.Wait()

			mu.Lock()
			defer mu.Unlock()
			if kills == 0 {
				t.Fatalf("no kills were done")
			}
			if wins < kills {
				t.Fatalf("did not win at least 10 elections")
			}
			if lost < kills {
				t.Fatalf("did not loose at least 10 leaderships")
			}
			if maxActive > 1 {
				t.Fatalf("had more than 1 active worker")
			}
			log.Printf("kills: %d, leaders elected: %d, leaderships lost: %d", kills, wins, lost)
		})
	}
}

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
