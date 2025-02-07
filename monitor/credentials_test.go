// Copyright 2024 The NATS Authors
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

package monitor_test

import (
	"os"
	"testing"

	"github.com/nats-io/jsm.go/monitor"
)

func TestCheckCredential(t *testing.T) {
	noExpiry := `-----BEGIN NATS USER JWT-----
eyJ0eXAiOiJKV1QiLCJhbGciOiJlZDI1NTE5LW5rZXkifQ.eyJqdGkiOiJBSUdIM0I2TEFGQkMzNktaSFJCSFI1QVZaTVFHQkdDS0NRTlNXRFBMN0U1NE5SM0I1SkxRIiwiaWF0IjoxNjk1MzY5NjU1LCJpc3MiOiJBRFFCT1haQTZaWk5MMko0VFpZNTZMUVpUN1FCVk9DNDVLVlQ3UDVNWkZVWU1LSVpaTUdaSE02QSIsIm5hbWUiOiJib2IiLCJzdWIiOiJVQkhPVDczREVGN1dZWUZUS1ZVSDZNWDNFUUVZSlFWWUNBRUJXUFJaSDNYR0E2WDdLRDNGUkFYSCIsIm5hdHMiOnsicHViIjp7fSwic3ViIjp7fSwic3VicyI6LTEsImRhdGEiOi0xLCJwYXlsb2FkIjotMSwidHlwZSI6InVzZXIiLCJ2ZXJzaW9uIjoyfX0.kGsxvI3NNNp60unItd-Eo1Yw6B9T3rBOeq7lvRY_klP5yTaBZwhCTKUNYdr_n2HNkCNB44fyW2_pmBhDki_CDQ
------END NATS USER JWT------

************************* IMPORTANT *************************
NKEY Seed printed below can be used to sign and prove identity.
NKEYs are sensitive and should be treated as secrets.

-----BEGIN USER NKEY SEED-----
SUAIQJDZJGYOJN4NBOLYRRENCNTPXZ7PPVQW7RWEXWJUNBAFDRPDO27JWA
------END USER NKEY SEED------

*************************************************************`

	expires2100 := `-----BEGIN NATS USER JWT-----
eyJ0eXAiOiJKV1QiLCJhbGciOiJlZDI1NTE5LW5rZXkifQ.eyJleHAiOjQxMDI0NDQ4MDAsImp0aSI6IlhRQkNTUUo3M0c3STRWR0JVUUNNQjdKRVlDWlVNUzdLUzJPU0Q1Skk3WjY0NEE0TU40SUEiLCJpYXQiOjE2OTUzNzA4OTcsImlzcyI6IkFERU5CTlBZSUwzTklXVkxCMjJVUU5FR0NMREhGSllNSUxEVEFQSlk1SFlQV05LQVZQNzJXREFSIiwibmFtZSI6ImJvYiIsInN1YiI6IlVCTTdYREtRUzRRQVBKUEFCSllWSU5RR1lETko2R043MjZNQ01DV0VZRDJTTU9GQVZOQ1E1M09IIiwibmF0cyI6eyJwdWIiOnt9LCJzdWIiOnt9LCJzdWJzIjotMSwiZGF0YSI6LTEsInBheWxvYWQiOi0xLCJ0eXBlIjoidXNlciIsInZlcnNpb24iOjJ9fQ.3ytewtkFoRLKNeRJjPGOyNWeeQKqKdfHmyRL2ofaUiqj_OoN2LAmg_Ms2zpU-A_2xAiUH7VsMIRJxw1cx3bwAg
------END NATS USER JWT------

************************* IMPORTANT *************************
NKEY Seed printed below can be used to sign and prove identity.
NKEYs are sensitive and should be treated as secrets.

-----BEGIN USER NKEY SEED-----
SUAKYITMHPMSYUGPNQBLLPGOPFQN44XNCGXHNSHLJJVMD3IKYGBOLAI7TI
------END USER NKEY SEED------

*************************************************************`

	writeCred := func(t *testing.T, cred string) string {
		tf, err := os.CreateTemp(t.TempDir(), "")
		assertNoError(t, err)

		tf.Write([]byte(cred))
		tf.Close()

		return tf.Name()
	}

	t.Run("no expiry", func(t *testing.T) {
		opts := monitor.CheckCredentialOptions{
			File:           writeCred(t, noExpiry),
			RequiresExpiry: true,
		}

		check := &monitor.Result{}
		assertNoError(t, monitor.CheckCredential(check, opts))
		assertListEquals(t, check.Criticals, "never expires")
		assertListIsEmpty(t, check.Warnings)

		opts.File = writeCred(t, expires2100)
		check = &monitor.Result{}
		assertNoError(t, monitor.CheckCredential(check, opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertListEquals(t, check.OKs, "expires in 2100-01-01 00:00:00 +0000 UTC")
	})

	t.Run("critical", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckCredential(check, monitor.CheckCredentialOptions{
			File:             writeCred(t, noExpiry),
			ValidityCritical: 100 * 24 * 365 * 60 * 60,
		}))
		assertListEquals(t, check.Criticals, "expires sooner than 100y0d0h0m0s")
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)
	})

	t.Run("warning", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckCredential(check, monitor.CheckCredentialOptions{
			File:            writeCred(t, noExpiry),
			ValidityWarning: 100 * 24 * 365 * 60 * 60,
		}))
		assertListEquals(t, check.Warnings, "expires sooner than 100y0d0h0m0s")
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.OKs)
	})
}
