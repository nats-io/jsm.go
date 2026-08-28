package registry

import (
	"sort"
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
var isWildcardSorted bool
var mu sync.Mutex

func RegisterTypeFactory(kind string, factory func() any) {
	mu.Lock()
	defer mu.Unlock()

	schemaTypes = append(schemaTypes, kind)
	factoryRegistry[kind] = factory
}

func RegisterResponseSubjectType(subj string, schemaType string) {
	mu.Lock()
	defer mu.Unlock()

	responseSubjectTypeRegistry[subj] = schemaType
}

func RegisterRequestSubjectType(subj string, schemaType string) {
	mu.Lock()
	defer mu.Unlock()

	requestSubjectTypeRegistry[subj] = schemaType
}

func RegisterWildcardType(subj string, schemaType string) {
	mu.Lock()
	defer mu.Unlock()

	wildcardSubjectTypeRegistry[subj] = schemaType
}

// lock must be held
func sortWildcardIfNotSorted() {
	if isWildcardSorted {
		return
	}

	wildcardSubjectsSorted = make([]string, 0, len(wildcardSubjectTypeRegistry))
	for k := range wildcardSubjectTypeRegistry {
		wildcardSubjectsSorted = append(wildcardSubjectsSorted, k)
	}
	sort.Strings(wildcardSubjectsSorted)

	isWildcardSorted = true
}
