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
	"os"
)

// envResolver reads secret material from an environment variable named
// in the reference: env://NAME. It is intended for containers or CI
// where credentials are injected as environment variables.
type envResolver struct{}

func (r *envResolver) Schemes() []string {
	return []string{"env"}
}

func (r *envResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
	name := trimSchemePrefix(ref, "env://")
	if name == "" {
		return nil, fmt.Errorf("env: missing variable name in reference %q", ref)
	}

	val, ok := os.LookupEnv(name)
	if !ok {
		return nil, fmt.Errorf("env: variable %q is not set", name)
	}

	if val == "" {
		return nil, fmt.Errorf("env: variable %q is empty", name)
	}

	return []byte(val), nil
}
