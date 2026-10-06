package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
	ntfclient "github.com/synadia-io/orbit.go/ntf-client"
)

func TestMain(m *testing.M) {
	RunWithNTF(m, 20000)
}

func withJSServer(t testing.TB, cb func(testing.TB, *nats.Conn, *jsm.Manager, *ntfclient.Instance)) {
	t.Helper()

	ntfc := ntfclient.New(t, NTFURL())
	defer ntfc.Close(t)
	ntfc.WithJetStreamServer(t, func(t testing.TB, nc *nats.Conn, instance *ntfclient.Instance) {
		mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
		checkErr(t, err, "create js manager failed")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, err := mgr.JetStreamAccountInfo()
				if err != nil {
					continue
				}

				cb(t, nc, mgr, instance)

				return
			case <-ctx.Done():
				t.Fatalf("jetstream did not become available")
			}
		}
	}, ntfclient.WithConnectOptions(nats.UseOldRequestStyle()))
}

func withJSServerWithAccounts(t *testing.T, accountsFile string, cb func(*ntfclient.Instance)) {
	t.Helper()

	accounts, err := os.ReadFile(accountsFile)
	if err != nil {
		t.Fatalf("could not read accounts file: %v", err)
	}

	ntfc := ntfclient.New(t, NTFURL())
	defer ntfc.Close(t)
	instance := ntfc.CreateServer(t, true, ntfclient.WithAccounts(string(accounts)), ntfclient.WithAuthorization("# no default user"))
	defer instance.Destroy(t)

	cb(instance)
}
