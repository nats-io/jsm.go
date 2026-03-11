package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewExamplesCollection(t *testing.T) {
	t.Run("zero limit creates unlimited collection", func(t *testing.T) {
		c := newExamplesCollection(0)
		if c.Limit != 0 {
			t.Errorf("expected Limit 0, got %d", c.Limit)
		}
		if c.Examples == nil {
			t.Error("expected non-nil Examples slice")
		}
		if len(c.Examples) != 0 {
			t.Errorf("expected empty Examples, got %d", len(c.Examples))
		}
	})

	t.Run("non-zero limit creates bounded collection", func(t *testing.T) {
		c := newExamplesCollection(5)
		if c.Limit != 5 {
			t.Errorf("expected Limit 5, got %d", c.Limit)
		}
		if len(c.Examples) != 0 {
			t.Errorf("expected empty Examples, got %d", len(c.Examples))
		}
	})
}

func TestExamplesCollectionAdd(t *testing.T) {
	t.Run("stores formatted examples", func(t *testing.T) {
		c := newExamplesCollection(0)
		c.Add("item %d", 1)
		c.Add("item %d", 2)
		if len(c.Examples) != 2 {
			t.Fatalf("expected 2 examples, got %d", len(c.Examples))
		}
		if c.Examples[0] != "item 1" {
			t.Errorf("expected 'item 1', got %q", c.Examples[0])
		}
		if c.Examples[1] != "item 2" {
			t.Errorf("expected 'item 2', got %q", c.Examples[1])
		}
	})

	t.Run("stores all when limit is zero", func(t *testing.T) {
		c := newExamplesCollection(0)
		for i := 0; i < 100; i++ {
			c.Add("item %d", i)
		}
		if len(c.Examples) != 100 {
			t.Errorf("expected 100 stored examples, got %d", len(c.Examples))
		}
		if c.Count() != 100 {
			t.Errorf("expected Count 100, got %d", c.Count())
		}
	})

	t.Run("stops storing at limit and counts the rest", func(t *testing.T) {
		c := newExamplesCollection(2)
		c.Add("a")
		c.Add("b")
		c.Add("c") // over limit, counted only
		c.Add("d") // over limit, counted only
		if len(c.Examples) != 2 {
			t.Errorf("expected 2 stored examples, got %d", len(c.Examples))
		}
		if c.Count() != 4 {
			t.Errorf("expected Count 4 (2 stored + 2 overflow), got %d", c.Count())
		}
	})

	t.Run("stores exactly the limit, no overflow", func(t *testing.T) {
		c := newExamplesCollection(3)
		c.Add("a")
		c.Add("b")
		c.Add("c")
		if len(c.Examples) != 3 {
			t.Errorf("expected 3 stored examples, got %d", len(c.Examples))
		}
		if c.Count() != 3 {
			t.Errorf("expected Count 3, got %d", c.Count())
		}
	})

	t.Run("is a no-op on nil receiver", func(t *testing.T) {
		var c *ExamplesCollection
		c.Add("item") // must not panic
	})
}

func TestExamplesCollectionCount(t *testing.T) {
	t.Run("returns zero for empty collection", func(t *testing.T) {
		c := newExamplesCollection(0)
		if c.Count() != 0 {
			t.Errorf("expected 0, got %d", c.Count())
		}
	})

	t.Run("returns stored count when no overflow", func(t *testing.T) {
		c := newExamplesCollection(0)
		c.Add("a")
		c.Add("b")
		if c.Count() != 2 {
			t.Errorf("expected 2, got %d", c.Count())
		}
	})

	t.Run("returns total including overflow", func(t *testing.T) {
		c := newExamplesCollection(2)
		c.Add("a")
		c.Add("b")
		c.Add("c") // overflow
		if c.Count() != 3 {
			t.Errorf("expected 3, got %d", c.Count())
		}
	})

	t.Run("returns zero on nil receiver", func(t *testing.T) {
		var c *ExamplesCollection
		if c.Count() != 0 {
			t.Errorf("expected 0 on nil, got %d", c.Count())
		}
	})
}

func TestExamplesCollectionClear(t *testing.T) {
	t.Run("resets stored examples", func(t *testing.T) {
		c := newExamplesCollection(0)
		c.Add("a")
		c.Add("b")
		c.Clear()
		if len(c.Examples) != 0 {
			t.Errorf("expected empty Examples after clear, got %d", len(c.Examples))
		}
		if c.Count() != 0 {
			t.Errorf("expected Count 0 after clear, got %d", c.Count())
		}
	})

	t.Run("resets overflow counter", func(t *testing.T) {
		c := newExamplesCollection(1)
		c.Add("a")
		c.Add("b") // overflow
		if c.Count() != 2 {
			t.Fatalf("expected Count 2 before clear, got %d", c.Count())
		}
		c.Clear()
		if c.Count() != 0 {
			t.Errorf("expected Count 0 after clear (overflow reset), got %d", c.Count())
		}
	})

	t.Run("is a no-op on nil receiver", func(t *testing.T) {
		var c *ExamplesCollection
		c.Clear() // must not panic
	})
}

func TestExamplesCollectionString(t *testing.T) {
	t.Run("returns empty string for empty collection", func(t *testing.T) {
		c := newExamplesCollection(0)
		if got := c.String(); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("formats each example with a dash prefix on its own line", func(t *testing.T) {
		c := newExamplesCollection(0)
		c.Add("foo")
		c.Add("bar")
		want := " - foo\n - bar\n"
		if got := c.String(); got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("appends overflow line when limit is exceeded", func(t *testing.T) {
		c := newExamplesCollection(2)
		c.Add("a")
		c.Add("b")
		c.Add("c")
		c.Add("d")
		want := " - a\n - b\n - ... and 2 more ...\n"
		if got := c.String(); got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("does not append overflow line when at exactly the limit", func(t *testing.T) {
		c := newExamplesCollection(2)
		c.Add("a")
		c.Add("b")
		want := " - a\n - b\n"
		if got := c.String(); got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("returns empty string on nil receiver", func(t *testing.T) {
		var c *ExamplesCollection
		if got := c.String(); got != "" {
			t.Errorf("expected empty string on nil, got %q", got)
		}
	})
}

func TestExamplesCollectionJSON(t *testing.T) {
	t.Run("empty error field is omitted from JSON", func(t *testing.T) {
		c := newExamplesCollection(0)
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if strings.Contains(string(b), `"error"`) {
			t.Errorf("expected error field to be omitted, got: %s", b)
		}
	})

	t.Run("non-empty error field appears in JSON", func(t *testing.T) {
		c := newExamplesCollection(0)
		c.Error = "something went wrong"
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if !strings.Contains(string(b), `"error":"something went wrong"`) {
			t.Errorf("expected error in JSON, got: %s", b)
		}
	})

	t.Run("empty examples slice is omitted from JSON", func(t *testing.T) {
		c := newExamplesCollection(0)
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if strings.Contains(string(b), `"examples"`) {
			t.Errorf("expected examples to be omitted, got: %s", b)
		}
	})

	t.Run("non-empty examples appear in JSON", func(t *testing.T) {
		c := newExamplesCollection(0)
		c.Add("one")
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if !strings.Contains(string(b), `"examples"`) {
			t.Errorf("expected examples in JSON, got: %s", b)
		}
	})

	t.Run("overflow counter is not serialized", func(t *testing.T) {
		c := newExamplesCollection(1)
		c.Add("a")
		c.Add("b") // overflow
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if strings.Contains(string(b), "overflow") {
			t.Errorf("expected overflow to be absent from JSON, got: %s", b)
		}
	})
}
