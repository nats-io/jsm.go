package registry_test

import (
	"testing"

	"github.com/nats-io/jsm.go/registry"
)

var benchAdvisory = []byte(`{
  "type": "io.nats.jetstream.advisory.v1.api_audit",
  "id": "uafvZ1UEDIW5FZV6kvLgWA",
  "timestamp": "2020-04-23T16:51:18.516363Z",
  "server": "NDF3LDJHQAOEA2FLLXCUCM6UUPQIRHRVYAT2WQBRXAAHNM5FQFCT2WUS",
  "client": {
    "host": "::1",
    "port": 57924,
    "cid": 17,
    "account": "$G",
    "name": "NATS CLI",
    "lang": "go",
    "version": "1.9.2"
  },
  "subject": "$JS.API.STREAM.CREATE.ORDERS",
  "request": "{\"name\":\"ORDERS\"}",
  "response": "{\"type\":\"io.nats.jetstream.api.v1.stream_create_response\"}"
}`)

// exercises the factory registry lookup, the shortest critical section and the
// one every parsed message passes through
func BenchmarkNewMessage(b *testing.B) {
	registry.NewMessage("io.nats.jetstream.api.v1.consumer_create_request")

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, ok := registry.NewMessage("io.nats.jetstream.api.v1.consumer_create_request")
			if !ok {
				b.Fatal("unknown type")
			}
		}
	})
}

// exercises the same lookup behind the JSON decode a real event handler does
func BenchmarkParseMessage(b *testing.B) {
	registry.ParseMessage(benchAdvisory)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := registry.ParseMessage(benchAdvisory)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// exercises the longest critical section, a collision scan over every
// registered wildcard subject
func BenchmarkTypeForRequestSubject(b *testing.B) {
	registry.TypeForRequestSubject("$JS.API.STREAM.INFO.ORDERS")

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := registry.TypeForRequestSubject("$JS.API.STREAM.INFO.ORDERS")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// exercises a miss, which scans every wildcard subject without matching
func BenchmarkTypeForRequestSubjectMiss(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := registry.TypeForRequestSubject("nope.nope.nope")
			if err == nil {
				b.Fatal("expected an error")
			}
		}
	})
}

func BenchmarkTypeForJetStreamRequestSubjectPrefix(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := registry.TypeForJetStreamRequestSubjectPrefix("$JS.API.STREAM.CREATE")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkTypeForJetStreamResponseSubjectPrefix(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := registry.TypeForJetStreamResponseSubjectPrefix("$JS.API.STREAM.CREATE")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// exercises a regular expression compile and a scan of every registered type
func BenchmarkSchemaSearch(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			found, err := registry.SchemaSearch("consumer_")
			if err != nil {
				b.Fatal(err)
			}
			if len(found) == 0 {
				b.Fatal("no matches")
			}
		}
	})
}
