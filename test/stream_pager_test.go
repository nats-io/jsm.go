// Copyright 2020-2024 The NATS Authors
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

package test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
)

func TestPager(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	str, err := mgr.NewStream("PAGERTEST", jsm.Subjects("js.in.pager"))
	if err != nil {
		t.Fatalf("stream create failed: %s", err)
	}

	for i := 1; i <= 200; i++ {
		_, err = nc.Request("js.in.pager", []byte(fmt.Sprintf("message %d", i)), time.Second)
		if err != nil {
			t.Fatalf("publish failed: %s", err)
		}
	}

	pgr, err := str.PageContents(jsm.PagerSize(25))
	if err != nil {
		t.Fatalf("pager creation failed: %s", err)
	}

	seen := 0
	pages := 0
	for {
		_, last, err := pgr.NextMsg(context.Background())
		if err != nil && last && seen == 200 && pages == 8 {
			break
		}

		if err != nil {
			t.Fatalf("next failed seen %d pages %d: %s", seen, pages, err)
		}

		seen++
		if last {
			pages++
		}
	}

	err = pgr.Close()
	if err != nil {
		t.Fatalf("close failed")
	}

	known, err := str.ConsumerNames()
	if err != nil {
		t.Fatalf("consumer named failed: %s", err)
	}
	if len(known) != 0 {
		t.Fatalf("expected no consumers got %v", known)
	}
}

func TestPagerDoubleClose(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	t.Run("consumer-based", func(t *testing.T) {
		str, err := mgr.NewStream("PAGERTEST", jsm.Subjects("js.in.pager"))
		checkErr(t, err, "stream create failed")

		_, err = nc.Request("js.in.pager", []byte("msg"), time.Second)
		checkErr(t, err, "publish failed")

		pgr, err := str.PageContents(jsm.PagerSize(25))
		checkErr(t, err, "pager creation failed")

		err = pgr.Close()
		checkErr(t, err, "first close failed")

		// second close must not panic and should be a no-op
		err = pgr.Close()
		if err != nil {
			t.Fatalf("second close returned unexpected error: %s", err)
		}
	})

	t.Run("direct-wq", func(t *testing.T) {
		str, err := mgr.NewStream("PAGERTEST_WQ", jsm.Subjects("js.in.pagerwq"), jsm.WorkQueueRetention(), jsm.AllowDirect())
		checkErr(t, err, "stream create failed")

		_, err = nc.Request("js.in.pagerwq", []byte("msg"), time.Second)
		checkErr(t, err, "publish failed")

		pgr, err := str.PageContents(jsm.PagerSize(25))
		checkErr(t, err, "pager creation failed")

		err = pgr.Close()
		checkErr(t, err, "first close failed")

		err = pgr.Close()
		if err != nil {
			t.Fatalf("second close returned unexpected error: %s", err)
		}
	})
}

func TestPagerCloseAfterConsumerDeleted(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	str, err := mgr.NewStream("PAGERTEST", jsm.Subjects("js.in.pager"))
	checkErr(t, err, "stream create failed")

	_, err = nc.Request("js.in.pager", []byte("msg"), time.Second)
	checkErr(t, err, "publish failed")

	pgr, err := str.PageContents(jsm.PagerSize(25))
	checkErr(t, err, "pager creation failed")

	// delete the underlying consumer out-of-band to force Delete() to fail in Close()
	names, err := str.ConsumerNames()
	checkErr(t, err, "consumer names failed")

	for _, name := range names {
		if strings.HasPrefix(name, "stream_pager_") {
			c, err := str.LoadConsumer(name)
			checkErr(t, err, "load consumer failed")
			err = c.Delete()
			checkErr(t, err, "pre-delete failed")
		}
	}

	// Close() should return an error (consumer already gone) but must not panic
	// and must complete the rest of cleanup
	err = pgr.Close()
	if err == nil {
		t.Fatal("expected error closing pager with pre-deleted consumer, got nil")
	}

	// second Close() must not panic — proves p.q and p.sub were nilled despite the error
	err = pgr.Close()
	if err != nil {
		t.Fatalf("second close returned unexpected error: %s", err)
	}
}

func TestPagerStartIdValidation(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	str, err := mgr.NewStream("PAGERTEST", jsm.Subjects("js.in.pager"))
	checkErr(t, err, "stream create failed")

	_, err = nc.Request("js.in.pager", []byte("msg"), time.Second)
	checkErr(t, err, "publish failed")

	for _, id := range []int{0, -2, -100} {
		_, err = str.PageContents(jsm.PagerStartId(id))
		if err == nil {
			t.Fatalf("PagerStartId(%d) should have returned an error", id)
		}
	}

	// valid sequence should succeed
	pgr, err := str.PageContents(jsm.PagerStartId(1))
	checkErr(t, err, "PagerStartId(1) should succeed")
	pgr.Close()
}

func TestPagerWQ(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	str, err := mgr.NewStream("PAGERTEST", jsm.Subjects("js.in.pager"), jsm.WorkQueueRetention(), jsm.AllowDirect())
	if err != nil {
		t.Fatalf("stream create failed: %s", err)
	}

	_, err = str.NewConsumer(jsm.ConsumerName("PULL"))
	if err != nil {
		t.Fatalf("consumer create failed: %s", err)
	}

	for i := 1; i <= 200; i++ {
		_, err = nc.Request("js.in.pager", []byte(fmt.Sprintf("message %d", i)), time.Second)
		if err != nil {
			t.Fatalf("publish failed: %s", err)
		}
	}

	_, err = str.PageContents(jsm.PagerSize(25), jsm.PagerStartDelta(time.Hour))
	if err == nil || err.Error() != "workqueue paging does not support time delta starting positions" {
		t.Fatalf("pager creation did not fail for time delta: %v", err)
	}

	pgr, err := str.PageContents(jsm.PagerSize(25))
	if err != nil {
		t.Fatalf("pager creation failed: %s", err)
	}

	seen := 0
	pages := 0
	for {
		_, last, err := pgr.NextMsg(context.Background())
		if err != nil && last && seen == 200 && pages == 8 {
			break
		}

		if err != nil {
			t.Fatalf("next failed seen %d pages %d: %s", seen, pages, err)
		}

		seen++
		if last {
			pages++
		}
	}

	err = pgr.Close()
	if err != nil {
		t.Fatalf("close failed")
	}
}
