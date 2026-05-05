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
	"encoding/base64"
	"fmt"
	"strings"
)

// EncodeDataURI returns a data:;base64,<payload> URI that round-trips
// through the data: resolver. It is intended for the Creds and NKey
// fields, whose payloads are multi-line decorated nkeys/JWT blobs that
// would otherwise be awkward to embed in JSON.
//
// The UserJwt, UserSeed, Token, and Password fields also accept a
// data: URI, but the underlying values are already single-line
// printable strings, so encoding them as data: provides no additional
// protection — base64 is not encryption. For secret-at-rest on those
// fields prefer an external resolver such as env://, op://, or a
// KV-backed store.
//
// Cert, Key, and CA are path-only and do not accept data: URIs.
func EncodeDataURI(payload []byte) string {
	return "data:;base64," + base64.StdEncoding.EncodeToString(payload)
}

// dataResolver decodes inline material embedded in a RFC 2397 data:
// URI, e.g. data:application/octet-stream;base64,<payload> or the
// shorter data:;base64,<payload>. A media type is optional and
// ignored. The ;base64 token is required — rejecting plain-text
// payloads avoids accidental byte-level surprises (newlines, URL
// escaping) when credentials are shuttled between systems.
type dataResolver struct{}

func (r *dataResolver) Schemes() []string {
	return []string{"data"}
}

func (r *dataResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
	const prefix = "data:"
	rest := trimSchemePrefix(ref, prefix)
	if rest == ref {
		return nil, fmt.Errorf("data: reference does not start with %q", prefix)
	}

	comma := strings.Index(rest, ",")
	if comma < 0 {
		return nil, fmt.Errorf("data: reference is missing the ',' separator")
	}
	header := rest[:comma]
	payload := rest[comma+1:]

	if !headerHasBase64(header) {
		return nil, fmt.Errorf("data: reference must declare ';base64' encoding")
	}

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("data: base64 decode failed: %w", err)
	}

	return decoded, nil
}

// headerHasBase64 reports whether the RFC 2397 header portion contains
// the ;base64 token. The header is semicolon-delimited and may carry a
// mediatype before the token: "application/octet-stream;base64" or
// simply ";base64".
func headerHasBase64(header string) bool {
	for _, part := range strings.Split(header, ";") {
		if strings.TrimSpace(part) == "base64" {
			return true
		}
	}

	return false
}
