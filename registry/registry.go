package registry

import (
	"slices"
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
}

func RegisterRequestSubjectType(subj string, schemaType string) {
	mu.Lock()
	defer mu.Unlock()

	requestSubjectTypeRegistry[subj] = schemaType
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
}
