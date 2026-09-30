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

package registry

import (
	"strings"
)

const apiPrefix = "$JS.API."

// NormalizeAPISubject removes a JetStream domain or API prefix from a request subject and returns the
// equivalent $JS.API subject. It is the reverse of jsm.APISubject for callers that do not know the
// prefix or domain that was used, jsm.StripAPISubject should be used when those are known.
//
//   - $JS.API.STREAM.INFO.ORDERS is returned as is
//   - $JS.hub.API.STREAM.INFO.ORDERS becomes $JS.API.STREAM.INFO.ORDERS
//   - JS.acc.API.STREAM.INFO.ORDERS becomes $JS.API.STREAM.INFO.ORDERS
//
// Prefixes are found by dropping tokens from the front of the subject until a token is reached that
// starts a subject known to the registry, like STREAM or CONSUMER. Subjects in $JS that are not in
// the $JS.<domain>.API form are not considered.
//
// The subject is not validated beyond that, for subjects that could not be normalized, or that hold
// wildcards, empty tokens or whitespace, the input is returned unchanged with ok false.
//
// This does not allocate when subject is already normalized or not recognized.
func NormalizeAPISubject(subject string) (normalized string, ok bool) {
	if !isLiteralSubject(subject) {
		return subject, false
	}

	if strings.HasPrefix(subject, apiPrefix) {
		return subject, true
	}

	// other than domains we do not consider prefixes in $JS to avoid matching things like advisories
	rest, found := strings.CutPrefix(subject, "$JS.")
	if found {
		_, rest, _ = strings.Cut(rest, ".")
		rest, found = strings.CutPrefix(rest, "API.")
		if found {
			return apiPrefix + rest, true
		}

		return subject, false
	}

	mu.RLock()
	defer mu.RUnlock()

	// the prefix is at least one token so we never consider the first token a verb
	_, rest, found = strings.Cut(subject, ".")
	for found {
		verb, next, more := strings.Cut(rest, ".")

		// $JS.API.INFO takes no arguments while INFO is common inside other subjects like STREAM.INFO.x
		if verb == "INFO" && more {
			rest = next
			continue
		}

		_, known := apiVerbs[verb]
		if known {
			return apiPrefix + rest, true
		}

		rest, found = next, more
	}

	return subject, false
}

// isLiteralSubject checks subject has no empty tokens, wildcards or whitespace
func isLiteralSubject(subject string) bool {
	if subject == "" {
		return false
	}

	prev := byte('.')
	for i := 0; i < len(subject); i++ {
		c := subject[i]
		switch c {
		case '.':
			if prev == '.' {
				return false
			}
		case '*', '>', ' ', '\t', '\r', '\n', '\x00':
			return false
		}
		prev = c
	}

	return prev != '.'
}
