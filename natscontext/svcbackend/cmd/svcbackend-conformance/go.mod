module github.com/nats-io/jsm.go/natscontext/svcbackend/cmd/svcbackend-conformance

go 1.25.0

// Transitional: svcbackend is not yet in a tagged jsm.go release, so
// builds must resolve against the parent checkout. Drop this replace
// and bump the jsm.go require once svcbackend ships in a tag.
replace github.com/nats-io/jsm.go => ../../../../

require (
	github.com/choria-io/fisk v0.9.1
	github.com/jedib0t/go-pretty/v6 v6.8.3
	github.com/nats-io/jsm.go v0.3.1-0.20260116154816-a772222ebcf0
	github.com/nats-io/nats.go v1.52.0
	github.com/nats-io/natscli v0.3.2
	github.com/nats-io/nkeys v0.4.16
	golang.org/x/term v0.45.0
)

require (
	github.com/antithesishq/antithesis-sdk-go v0.7.2 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/minio/highwayhash v1.0.4 // indirect
	github.com/nats-io/jwt/v2 v2.8.2 // indirect
	github.com/nats-io/nats-server/v2 v2.14.3 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
