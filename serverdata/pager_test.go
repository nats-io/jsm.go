package serverdata

import (
	"testing"

	"github.com/nats-io/nats-server/v2/server"
)

func TestJszPager(t *testing.T) {
	t.Run("passing no options should set default limit", func(t *testing.T) {
		p := jszPager(server.JszEventOptions{})
		if p.Limit != 32 {
			t.Fatal("account default limit not set to 32")
		}
	})

	t.Run("passing options should keep limit", func(t *testing.T) {
		want := 5
		p := jszPager(server.JszEventOptions{
			JSzOptions: server.JSzOptions{
				Limit: want,
			},
		})
		if p.Limit != want {
			t.Fatalf("got account limit %d, want %d", p.Limit, want)
		}
	})

	t.Run("limit less than zero should default to page size", func(t *testing.T) {
		p := jszPager(server.JszEventOptions{
			JSzOptions: server.JSzOptions{
				Limit: -1,
			},
		})
		if p.Limit != 32 {
			t.Fatalf("got account limit %d, want %d", p.Limit, 32)
		}
	})

	t.Run("Accounts option should be true, incoming offset should be kept", func(t *testing.T) {
		p := jszPager(server.JszEventOptions{
			JSzOptions: server.JSzOptions{
				Accounts: false,
			},
		})
		want := 64
		opts := p.Opts(want).(server.JszEventOptions)
		if !opts.Accounts {
			t.Fatalf("accounts option should be true")
		}
		if opts.Offset != want {
			t.Fatalf("got offset %d want %d", opts.Offset, want)
		}
	})
}

func TestSubszPager(t *testing.T) {
	t.Run("overrides default limit", func(t *testing.T) {
		if p := subszPager(server.SubszEventOptions{}); p.Limit != 1024 {
			t.Fatalf("limit = %d, want 1024", p.Limit)
		}

		in := server.SubszEventOptions{}
		in.SubszOptions.Limit = 10
		p := subszPager(in)
		if p.Limit != 10 {
			t.Fatalf("limit=%d, want=%d", p.Limit, 10)
		}
	})
	t.Run("Subscriptions option should be true", func(t *testing.T) {
		in := server.SubszEventOptions{}
		got := subszPager(in).Opts(1025).(server.SubszEventOptions)
		if !got.Subscriptions {
			t.Fatal("subscriptions option should be true")
		}
	})

	t.Run("opts sets offset on a copy and carries the limit", func(t *testing.T) {
		in := server.SubszEventOptions{}
		got := subszPager(in).Opts(64).(server.SubszEventOptions)
		if got.SubszOptions.Offset != 64 || got.SubszOptions.Limit != 1024 {
			t.Fatalf("got offset=%d limit=%d", got.SubszOptions.Offset, got.SubszOptions.Limit)
		}
		if in.SubszOptions.Offset != 0 {
			t.Fatal("caller opts mutated")
		}
	})
}

func TestConnzPager(t *testing.T) {
	t.Run("overrides default limit", func(t *testing.T) {
		if p := connzPager(server.ConnzEventOptions{}); p.Limit != 1024 {
			t.Fatalf("limit=%d, want=%d", p.Limit, 1024)
		}

		in := server.ConnzEventOptions{}
		in.ConnzOptions.Limit = 10
		p := connzPager(in)
		if p.Limit != 10 {
			t.Fatalf("limit=%d, want=%d", p.Limit, 10)
		}
	})

	t.Run("opts sets offset on a copy and carries the limit", func(t *testing.T) {
		in := server.ConnzEventOptions{}
		got := connzPager(in).Opts(64).(server.ConnzEventOptions)
		if got.ConnzOptions.Offset != 64 || got.ConnzOptions.Limit != 1024 {
			t.Fatalf("got offset=%d limit=%d", got.ConnzOptions.Offset, got.ConnzOptions.Limit)
		}
		if in.ConnzOptions.Offset != 0 {
			t.Fatal("caller opts mutated")
		}
	})
}
