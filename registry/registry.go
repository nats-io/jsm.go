package registry

import (
	"slices"
	"strings"
	"sync"
)

var factoryRegistry = map[string]func() any{
	"io.nats.unknown_message": func() any { return &UnknownMessage{} },
}
var wildcardSubjectTypeRegistry = map[string]string{}
var responseSubjectTypeRegistry = map[string]string{}
var requestSubjectTypeRegistry = map[string]string{}

var schemaTypes = []string{}
var wildcardSubjectsSorted []string
var apiVerbs = map[string]struct{}{}
var mu sync.RWMutex

func RegisterTypeFactory(kind string, factory func() any) {
	mu.Lock()
	defer mu.Unlock()

	_, known := factoryRegistry[kind]
	if !known {
		schemaTypes = append(schemaTypes, kind)
	}

	factoryRegistry[kind] = factory
}

func RegisterResponseSubjectType(subj string, schemaType string) {
	mu.Lock()
	defer mu.Unlock()

	responseSubjectTypeRegistry[subj] = schemaType
	registerAPIVerbLocked(subj)
}

func RegisterRequestSubjectType(subj string, schemaType string) {
	mu.Lock()
	defer mu.Unlock()

	requestSubjectTypeRegistry[subj] = schemaType
	registerAPIVerbLocked(subj)
}

func RegisterWildcardType(subj string, schemaType string) {
	mu.Lock()
	defer mu.Unlock()

	_, known := wildcardSubjectTypeRegistry[subj]
	wildcardSubjectTypeRegistry[subj] = schemaType

	if !known {
		i, _ := slices.BinarySearch(wildcardSubjectsSorted, subj)
		wildcardSubjectsSorted = slices.Insert(wildcardSubjectsSorted, i, subj)
	}

	registerAPIVerbLocked(subj)
}

// records the first token following $JS.API. in subj, write lock must be held
func registerAPIVerbLocked(subj string) {
	rest, ok := strings.CutPrefix(subj, apiPrefix)
	if !ok {
		return
	}

	verb, _, _ := strings.Cut(rest, ".")
	apiVerbs[verb] = struct{}{}
}
