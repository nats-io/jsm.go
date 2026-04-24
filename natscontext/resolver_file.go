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
	"os"
)

// fileResolver handles file:// URIs. Bare paths for Creds/NKey are
// routed directly to nats.UserCredentials / nats.NkeyOptionFromSeed by
// the short-circuits in buildCredsOption / buildNkeyOption without
// consulting the resolver map, so fileResolver deliberately does not
// claim the empty scheme: that slot is reserved for a user-supplied
// fallback resolver for bare Token/Password/UserJwt/UserSeed values.
type fileResolver struct{}

func (r *fileResolver) Schemes() []string {
	return []string{"file"}
}

func (r *fileResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
	path := filePathFromRef(ref)
	return os.ReadFile(path)
}

// filePathFromRef strips a file:// prefix if present and applies
// expandHomedir so ~ and $VAR expansion match the pre-refactor
// behavior for raw paths.
func filePathFromRef(ref string) string {
	return expandHomedir(trimSchemePrefix(ref, "file://"))
}
