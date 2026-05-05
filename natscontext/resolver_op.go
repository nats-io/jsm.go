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
	"context"
	"fmt"
	"os/exec"
)

// opResolver shells out to the 1Password CLI (`op`) to resolve secret
// references of the form op://vault/item/field. It replaces the ad-hoc
// dispatch that previously lived inline in Context.NATSOptions.
type opResolver struct{}

func (r *opResolver) Schemes() []string {
	return []string{"op"}
}

func (r *opResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
	const prefix = "op://"
	rest := trimSchemePrefix(ref, prefix)
	if rest != ref {
		ref = prefix + rest
	}
	cmd := exec.CommandContext(ctx, "op", "read", ref)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("op read failed: %w", err)
	}

	return out, nil
}
