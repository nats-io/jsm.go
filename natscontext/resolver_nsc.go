// Copyright 2026 The NATS Authors
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

package natscontext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// nscResolver shells out to the NATS `nsc` CLI to resolve references of
// the form nsc://<operator>/<account>/<user>. The reference path is
// passed through to `nsc generate profile`, whose JSON output names a
// credentials file whose bytes are returned to the caller. It also
// backs the deprecated settings.NSCLookup load-time path.
type nscResolver struct{}

func (r *nscResolver) Schemes() []string {
	return []string{"nsc"}
}

func (r *nscResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
	profile, err := runNscProfile(ctx, ref)
	if err != nil {
		return nil, err
	}
	if profile.UserCreds == "" {
		return nil, fmt.Errorf("nsc output did not include user_creds")
	}
	return os.ReadFile(profile.UserCreds)
}

// nscProfile holds the subset of `nsc generate profile` output we
// consume. The command emits additional fields we intentionally
// ignore.
type nscProfile struct {
	UserCreds string
	Service   string // service URL(s), comma-joined
}

type nscProfileRaw struct {
	UserCreds string `json:"user_creds"`
	Operator  struct {
		Service []string `json:"service"`
	} `json:"operator"`
}

// runNscProfile invokes `nsc generate profile <ref>` and parses the
// JSON output. It is shared between the nsc:// resolver, the
// load-time NSCLookup field handling, and the save-time migration so
// every path produces identical results. ref may be the URI form
// (nsc://op/acct/user) or bare form (op/acct/user); a leading
// nsc:// is stripped before invoking the CLI. Stdout is never
// included in returned errors because it can carry decorated JWTs.
func runNscProfile(ctx context.Context, ref string) (*nscProfile, error) {
	path, err := exec.LookPath("nsc")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'nsc' in user path")
	}

	ref = trimSchemePrefix(ref, "nsc://")
	cmd := exec.CommandContext(ctx, path, "generate", "profile", ref)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		hint := strings.TrimSpace(stderr.String())
		if hint == "" {
			return nil, fmt.Errorf("nsc invoke failed: %w", err)
		}
		return nil, fmt.Errorf("nsc invoke failed: %w: %s", err, hint)
	}

	var raw nscProfileRaw
	err = json.Unmarshal(out, &raw)
	if err != nil {
		return nil, fmt.Errorf("could not parse nsc output: %w", err)
	}

	return &nscProfile{
		UserCreds: raw.UserCreds,
		Service:   strings.Join(raw.Operator.Service, ","),
	}, nil
}
