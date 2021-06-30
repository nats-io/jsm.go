// Copyright 2021 The NATS Authors
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

package kv

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

var (
	validKeyRe    = regexp.MustCompile(ValidKeyPattern)
	validBucketRe = regexp.MustCompile(ValidBucketPattern)
)

// Operation is the type of action taken on a key
type Operation string

const (
	// PutOperation represents a PUT of data into the kv bucket
	PutOperation Operation = "PUT"

	// DeleteOperation represents a delete of a specific key in a bucket
	DeleteOperation Operation = "DEL"

	// ValidKeyPattern is a regular expression key should match, post encoding, to be valid
	ValidKeyPattern = `\A[-/_a-zA-Z0-9]+\z`

	// ValidBucketPattern is a regular expression bucket names should match
	ValidBucketPattern = `\A[a-zA-Z0-9_-]+\z`

	// ReservedKeyPrefix is a prefix for keys reserved for internal use
	ReservedKeyPrefix = "_kv"

	delOperationString string = string(DeleteOperation)
	putOperationString string = string(PutOperation)
)

var (
	// ErrUnknownKey is returned when a key does not exist
	ErrUnknownKey = errors.New("unknown key")

	// ErrInvalidKey is returned for keys that do not match ValidKeyPattern
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidBucketName is returned when trying to access buckets that do not match ValidBucketPattern
	ErrInvalidBucketName = errors.New("invalid bucket name")
)

// NewBucket creates or load a bucket. If the bucket already exist the existing bucket configuration is not reconciled
func NewBucket(nc *nats.Conn, bucket string, opts ...Option) (KV, error) {
	return newOrLoad(nc, bucket, true, opts...)
}

// NewClient creates a new read-write client
func NewClient(nc *nats.Conn, bucket string, opts ...Option) (KV, error) {
	return newOrLoad(nc, bucket, false, opts...)
}

// NewRoClient creates a read only key value store.
//
// NOTE: for now this is just a client, but this is reserved for read replicas use cases
func NewRoClient(nc *nats.Conn, bucket string, opts ...Option) (RoKV, error) {
	return NewClient(nc, bucket, opts...)
}

func newOrLoad(nc *nats.Conn, bucket string, create bool, opts ...Option) (KV, error) {
	o, err := newOpts(opts...)
	if err != nil {
		return nil, err
	}

	store, err := newJetStreamStorage(bucket, nc, o)
	if err != nil {
		return nil, err
	}

	if create {
		err = store.CreateBucket()
		if err != nil {
			return nil, err
		}
	}

	if o.localCache {
		return newReadCache(store, o.log)
	}

	return store, nil
}

type Storage interface {
	KV

	Bucket() string
	BucketSubject() string
	CreateBucket() error
}

// KV is a read-write interface to a single key-value store bucket
type KV interface {
	// Put saves a value into a key
	Put(key string, val string, opts ...PutOption) (seq uint64, err error)

	// Delete purges the subject
	Delete(key string) error

	// Destroy removes the entire bucket and all data, KV cannot be used after
	Destroy() error

	// Purge removes all data from the bucket but leaves the bucket
	Purge() error

	RoKV
}

// RoKV is a read-only interface to a single key-value store bucket
type RoKV interface {
	// Get gets a key from the store
	Get(key string) (Result, error)

	// History retrieves historic values for a key
	History(ctx context.Context, key string) ([]Result, error)

	// WatchBucket watches the entire bucket for changes, all keys and values will be traversed including all historic values
	WatchBucket(ctx context.Context) (Watch, error)

	// Watch a key for updates, the same Result might be delivered more than once
	Watch(ctx context.Context, key string) (Watch, error)

	// Close releases in-memory resources held by the KV, called automatically if the context used to create it is canceled
	Close() error

	// JSON dumps the entire KV as k=v values in JSON format
	JSON(ctx context.Context) ([]byte, error)

	// Status retrieves the status of the bucket
	Status() (Status, error)
}

// Codec encodes/decodes values using Encoders and Decoders
type Codec interface {
	Encoder
	Decoder
}

// Encoder encodes values before saving
type Encoder interface {
	Encode(value string) string
}

// Decoder decodes values before saving
type Decoder interface {
	Decode(value string) string
}

type Result interface {
	// Bucket is the bucket the data was loaded from
	Bucket() string
	// Key is the key that was retrieved
	Key() string
	// Value is the retrieved value
	Value() string
	// Created is the time the data was received in the bucket
	Created() time.Time
	// Sequence is a unique sequence for this value
	Sequence() uint64
	// Delta is distance from the latest value. If history is enabled this is effectively the index of the historical value, 0 for latest, 1 for most recent etc.
	Delta() uint64
	// OriginCluster is the cluster where this data originate from, may be empty if sharing was not enabled
	OriginCluster() string
	// Operation is the kind of operation this result represents
	Operation() Operation
}

// GenericResult is a generic, non implementation specific, representation of a Result
type GenericResult struct {
	Bucket        string `json:"bucket"`
	Key           string `json:"key"`
	Val           string `json:"val"`
	Created       int64  `json:"created"`
	Seq           uint64 `json:"seq"`
	Operation     string `json:"operation"`
	OriginCluster string `json:"origin_cluster"`
}

// Watch observes a bucket and report any changes via NextValue or Channel
type Watch interface {
	// Channel returns a channel to read changes from
	Channel() chan Result

	// Close must be called to dispose of resources, called if the context used to create the watch is canceled
	Close() error
}

type Status interface {
	// Bucket the name of the bucket
	Bucket() string

	// Values is how many messages are in the bucket, including historical values
	Values() uint64

	// History returns the configured history kept per key
	History() int64

	// TTL is how long the bucket keeps values for
	TTL() time.Duration

	// Cluster returns the name of the cluster holding the read replica of the data
	Cluster() string

	// Replicas returns how many times data in the bucket is replicated at storage
	Replicas() (ok int, failed int)

	// Keys returns a list of all keys in the bucket - not possible now
	Keys() ([]string, error)

	// BackingStore is a backend specific name for the underlying storage - eg. stream name
	BackingStore() string

	// MirrorStatus is the status of a read replica, error when not accessing a replica
	MirrorStatus() (lag int64, active bool, err error)

	// MaxBucketSize is the configured maximum size of the bucket in bytes
	MaxBucketSize() int64

	// MaxValueSize is the configured maximum size of a single value in bytes
	MaxValueSize() int32
}

// IsReservedKey determines if key is a reserved key
func IsReservedKey(key string) bool {
	return strings.HasPrefix(key, ReservedKeyPrefix)
}

// IsValidKey determines if key is a valid key
func IsValidKey(key string) bool {
	return validKeyRe.MatchString(key)
}

// IsValidBucket determines if bucket is a valid bucket name
func IsValidBucket(bucket string) bool {
	return validBucketRe.MatchString(bucket)
}
