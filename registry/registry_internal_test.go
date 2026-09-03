// Copyright 2025 The NATS Authors
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
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/nats-io/nats-server/v2/server"
)

// snapshotRegistry copies the package level registries and restores them when the test finishes
func snapshotRegistry(t *testing.T) {
	t.Helper()

	mu.Lock()
	defer mu.Unlock()

	factories := maps.Clone(factoryRegistry)
	wildcards := maps.Clone(wildcardSubjectTypeRegistry)
	responses := maps.Clone(responseSubjectTypeRegistry)
	requests := maps.Clone(requestSubjectTypeRegistry)
	types := slices.Clone(schemaTypes)
	sorted := slices.Clone(wildcardSubjectsSorted)

	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()

		factoryRegistry = factories
		wildcardSubjectTypeRegistry = wildcards
		responseSubjectTypeRegistry = responses
		requestSubjectTypeRegistry = requests
		schemaTypes = types
		wildcardSubjectsSorted = sorted
	})
}

func TestRequestSubjectRegistry(t *testing.T) {
	mu.Lock()
	subjects := maps.Clone(requestSubjectTypeRegistry)
	mu.Unlock()

	if len(subjects) == 0 {
		t.Fatal("no request subjects registered")
	}

	for prefix, schemaType := range subjects {
		v, err := TypeForJetStreamRequestSubjectPrefix(prefix)
		if err != nil {
			t.Errorf("%s: %s", prefix, err)
			continue
		}

		instance, ok := v.(SchemaManagedApiRequestType)
		if !ok {
			t.Errorf("%s: expected SchemaManagedApiRequestType got %T", prefix, v)
			continue
		}

		if instance.SchemaType() != schemaType {
			t.Errorf("%s: registered as %q but reports %q", prefix, schemaType, instance.SchemaType())
		}

		reported, err := instance.ApiSubjectPrefix()
		if err != nil {
			t.Errorf("%s: %s", prefix, err)
			continue
		}

		if reported != prefix {
			t.Errorf("%s: ApiSubjectPrefix() reports %q", prefix, reported)
		}
	}
}

func TestResponseSubjectRegistry(t *testing.T) {
	mu.Lock()
	subjects := maps.Clone(responseSubjectTypeRegistry)
	mu.Unlock()

	if len(subjects) == 0 {
		t.Fatal("no response subjects registered")
	}

	for prefix, schemaType := range subjects {
		v, err := TypeForJetStreamResponseSubjectPrefix(prefix)
		if err != nil {
			t.Errorf("%s: %s", prefix, err)
			continue
		}

		instance, ok := v.(SchemaManagedType)
		if !ok {
			t.Errorf("%s: expected SchemaManagedType got %T", prefix, v)
			continue
		}

		if instance.SchemaType() != schemaType {
			t.Errorf("%s: registered as %q but reports %q", prefix, schemaType, instance.SchemaType())
		}
	}
}

func TestWildcardSubjectRegistry(t *testing.T) {
	mu.Lock()
	subjects := maps.Clone(wildcardSubjectTypeRegistry)
	mu.Unlock()

	if len(subjects) == 0 {
		t.Fatal("no wildcard subjects registered")
	}

	for subject, schemaType := range subjects {
		v, ok := NewMessage(schemaType)
		if !ok {
			t.Errorf("%s: no factory for %q", subject, schemaType)
			continue
		}

		instance, ok := v.(SchemaManagedApiRequestType)
		if !ok {
			t.Errorf("%s: expected SchemaManagedApiRequestType got %T", subject, v)
			continue
		}

		pattern, err := instance.ApiSubjectPattern()
		if err != nil {
			t.Errorf("%s: %s", subject, err)
			continue
		}

		if pattern != subject {
			t.Errorf("%s: ApiSubjectPattern() reports %q", subject, pattern)
		}

		format, err := instance.ApiSubjectFormat()
		if err != nil {
			t.Errorf("%s: %s", subject, err)
			continue
		}

		tokens := wildcardTokens(subject)
		if strings.Count(format, "%s") != tokens {
			t.Errorf("%s: pattern has %d wildcard tokens but format %q has %d", subject, tokens, format, strings.Count(format, "%s"))
			continue
		}

		args := make([]any, tokens)
		for i := range args {
			args[i] = fmt.Sprintf("token%d", i)
		}

		concrete := fmt.Sprintf(format, args...)
		matched, err := TypeForRequestSubject(concrete)
		if err != nil {
			t.Errorf("%s: %s does not resolve: %s", subject, concrete, err)
			continue
		}

		resolved, ok := matched.(SchemaManagedType)
		if !ok {
			t.Errorf("%s: %s resolved to %T", subject, concrete, matched)
			continue
		}

		if resolved.SchemaType() != schemaType {
			t.Errorf("%s: %s resolved to %q", subject, concrete, resolved.SchemaType())
		}
	}
}

// TypeForRequestSubject returns the first colliding wildcard in sorted order, any collision
// between registered wildcards would make that choice arbitrary
func TestWildcardSubjectsDoNotCollide(t *testing.T) {
	mu.Lock()
	subjects := slices.Sorted(maps.Keys(wildcardSubjectTypeRegistry))
	mu.Unlock()

	for i := 0; i < len(subjects); i++ {
		for j := i + 1; j < len(subjects); j++ {
			if server.SubjectsCollide(subjects[i], subjects[j]) {
				t.Errorf("%q and %q collide", subjects[i], subjects[j])
			}
		}
	}
}

func wildcardTokens(subject string) int {
	var tokens int
	for _, token := range strings.Split(subject, ".") {
		if token == "*" || token == ">" {
			tokens++
		}
	}

	return tokens
}

func TestRegisterTypeFactory(t *testing.T) {
	snapshotRegistry(t)

	RegisterTypeFactory("io.nats.test.v1.factory", func() any { return &UnknownMessage{} })

	v, ok := NewMessage("io.nats.test.v1.factory")
	if !ok {
		t.Fatal("expected the registered factory to be found")
	}

	if _, ok := v.(*UnknownMessage); !ok {
		t.Fatalf("expected *UnknownMessage got %T", v)
	}

	found, err := SchemaSearch("^io\\.nats\\.test\\.v1\\.factory$")
	if err != nil {
		t.Fatalf("search failed: %s", err)
	}

	if len(found) != 1 {
		t.Fatalf("expected the type to be searchable, got %v", found)
	}
}

func TestRegisterRequestSubjectType(t *testing.T) {
	snapshotRegistry(t)

	RegisterTypeFactory("io.nats.test.v1.request", func() any { return &UnknownMessage{} })
	RegisterRequestSubjectType("TEST.REQUEST", "io.nats.test.v1.request")

	v, err := TypeForJetStreamRequestSubjectPrefix("TEST.REQUEST")
	if err != nil {
		t.Fatalf("lookup failed: %s", err)
	}

	if _, ok := v.(*UnknownMessage); !ok {
		t.Fatalf("expected *UnknownMessage got %T", v)
	}
}

func TestRegisterResponseSubjectType(t *testing.T) {
	snapshotRegistry(t)

	RegisterTypeFactory("io.nats.test.v1.response", func() any { return &UnknownMessage{} })
	RegisterResponseSubjectType("TEST.RESPONSE", "io.nats.test.v1.response")

	v, err := TypeForJetStreamResponseSubjectPrefix("TEST.RESPONSE")
	if err != nil {
		t.Fatalf("lookup failed: %s", err)
	}

	if _, ok := v.(*UnknownMessage); !ok {
		t.Fatalf("expected *UnknownMessage got %T", v)
	}
}

// RegisterWildcardType has to invalidate the cache TypeForRequestSubject builds, else
// anything registered after the first lookup is never matched
func TestRegisterWildcardType(t *testing.T) {
	snapshotRegistry(t)

	_, err := TypeForRequestSubject("$JS.API.STREAM.CREATE.ORDERS")
	if err != nil {
		t.Fatalf("could not warm the wildcard cache: %s", err)
	}

	RegisterTypeFactory("io.nats.test.v1.wildcard", func() any { return &UnknownMessage{} })
	RegisterWildcardType("TEST.WILDCARD.*", "io.nats.test.v1.wildcard")

	v, err := TypeForRequestSubject("TEST.WILDCARD.thing")
	if err != nil {
		t.Fatalf("wildcard registered after the first lookup was not matched: %s", err)
	}

	if _, ok := v.(*UnknownMessage); !ok {
		t.Fatalf("expected *UnknownMessage got %T", v)
	}
}

// A kind registered twice should appear once in the searchable types
func TestRegisterTypeFactoryDoesNotDuplicate(t *testing.T) {
	snapshotRegistry(t)

	factory := func() any { return &UnknownMessage{} }
	RegisterTypeFactory("io.nats.test.v1.duplicate", factory)
	RegisterTypeFactory("io.nats.test.v1.duplicate", factory)

	found, err := SchemaSearch("^io\\.nats\\.test\\.v1\\.duplicate$")
	if err != nil {
		t.Fatalf("search failed: %s", err)
	}

	if len(found) != 1 {
		t.Fatalf("expected 1 entry got %v", found)
	}
}

// The failure to build the request type has to be reported as a request failure
func TestTypeForJetStreamRequestSubjectPrefixWithoutFactory(t *testing.T) {
	snapshotRegistry(t)

	RegisterRequestSubjectType("TEST.NOFACTORY", "io.nats.test.v1.no_factory")

	_, err := TypeForJetStreamRequestSubjectPrefix("TEST.NOFACTORY")
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "request") {
		t.Fatalf("expected an error about the request subject got %q", err)
	}
}

func TestConcurrentRegistrationAndLookup(t *testing.T) {
	snapshotRegistry(t)

	const workers = 8

	var wg sync.WaitGroup

	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			kind := fmt.Sprintf("io.nats.test.v1.concurrent_%d", i)
			RegisterTypeFactory(kind, func() any { return &UnknownMessage{} })
			RegisterRequestSubjectType(fmt.Sprintf("TEST.CONCURRENT.%d", i), kind)
			RegisterResponseSubjectType(fmt.Sprintf("TEST.CONCURRENT.%d", i), kind)
			RegisterWildcardType(fmt.Sprintf("TEST.CONCURRENT.%d.*", i), kind)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			NewMessage("io.nats.jetstream.api.v1.stream_create_request")
			SchemaSearch("stream_create")
			TypeForRequestSubject("$JS.API.STREAM.CREATE.ORDERS")
			TypeForJetStreamRequestSubjectPrefix("$JS.API.STREAM.CREATE")
			TypeForJetStreamResponseSubjectPrefix("$JS.API.STREAM.CREATE")
		}()
	}

	wg.Wait()

	for i := range workers {
		kind := fmt.Sprintf("io.nats.test.v1.concurrent_%d", i)
		_, ok := NewMessage(kind)
		if !ok {
			t.Errorf("%s was not registered", kind)
		}
	}
}

func TestTypeForJetStreamResponseSubjectPrefixWithoutFactory(t *testing.T) {
	snapshotRegistry(t)

	RegisterResponseSubjectType("TEST.NOFACTORY", "io.nats.test.v1.no_factory")

	_, err := TypeForJetStreamResponseSubjectPrefix("TEST.NOFACTORY")
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "response") {
		t.Fatalf("expected an error about the response subject got %q", err)
	}
}

func TestTypeForRequestSubjectWithoutFactory(t *testing.T) {
	snapshotRegistry(t)

	RegisterWildcardType("TEST.NOFACTORY.*", "io.nats.test.v1.no_factory")

	_, err := TypeForRequestSubject("TEST.NOFACTORY.thing")
	if err == nil {
		t.Fatal("expected an error")
	}
}
