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

package monitor

import (
	"os"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

// CheckCredentialOptions configures the credentials check
type CheckCredentialOptions struct {
	// File is the file holding the credential
	File string `json:"file" yaml:"file"`
	// ValidityWarning is the warning threshold for credential validity (seconds)
	ValidityWarning float64 `json:"validity_warning" yaml:"validity_warning"`
	// ValidityCritical is the critical threshold for credential validity (seconds)
	ValidityCritical float64 `json:"validity_critical" yaml:"validity_critical"`
	// RequiresExpiry requires the credential to have a validity set
	RequiresExpiry bool `json:"requires_expiry" yaml:"requires_expiry"`
}

func CheckCredential(check *Result, opts CheckCredentialOptions) error {
	ok, err := fileAccessible(opts.File)
	if err != nil {
		check.Criticalf("credential not accessible: %v", err)
		return nil
	}

	if !ok {
		check.Critical("credential not accessible")
		return nil
	}

	cb, err := os.ReadFile(opts.File)
	if err != nil {
		check.Criticalf("credential not accessible: %v", err)
		return nil
	}

	token, err := nkeys.ParseDecoratedJWT(cb)
	if err != nil {
		check.Criticalf("invalid credential: %v", err)
		return nil
	}

	claims, err := jwt.Decode(token)
	if err != nil {
		check.Criticalf("invalid credential: %v", err)
		return nil
	}

	now := time.Now().UTC().Unix()
	cd := claims.Claims()
	until := cd.Expires - now
	crit := int64(secondsToDuration(opts.ValidityCritical))
	warn := int64(secondsToDuration(opts.ValidityWarning))

	check.Pd(&PerfDataItem{Help: "Expiry time in seconds", Name: "expiry", Value: float64(until), Warn: float64(warn), Crit: float64(crit), Unit: "s"})

	switch {
	case cd.Expires == 0 && opts.RequiresExpiry:
		check.Critical("never expires")
	case opts.ValidityCritical > 0 && (until <= crit):
		check.Criticalf("expires sooner than %s", f(secondsToDuration(opts.ValidityCritical)))
	case opts.ValidityWarning > 0 && (until <= warn):
		check.Warnf("expires sooner than %s", f(secondsToDuration(opts.ValidityWarning)))
	default:
		if cd.Expires == 0 {
			check.Ok("never expires")
		} else {
			check.Okf("expires in %s", time.Unix(cd.Expires, 0).UTC())
		}
	}

	return nil
}
