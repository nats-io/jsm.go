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
	"strings"
)

// fileResolver handles file:// URIs and bare paths. Bare paths are the
// historical default for Creds/NKey fields; the empty scheme keeps
// backward compatibility with those values.
type fileResolver struct{}

func (r *fileResolver) Schemes() []string {
	return []string{"file", ""}
}

func (r *fileResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
	path := filePathFromRef(ref)
	return os.ReadFile(path)
}

// filePathFromRef strips a file:// prefix if present and applies
// expandHomedir so ~ and $VAR expansion match the pre-refactor
// behavior for raw paths.
func filePathFromRef(ref string) string {
	return expandHomedir(strings.TrimPrefix(ref, "file://"))
}
