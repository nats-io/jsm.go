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

package backup

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

const (
	keyFileSuffix  = ".keys.json"
	keyFileVersion = 1
	secretBytes    = 32
	tokenBytes     = 10
	objStorePrefix = "OBJ_"
	kvSubjectRoot  = "$KV"
	objSubjectRoot = "$O"
)

var tokenEncoding = base32.HexEncoding.WithPadding(base32.NoPadding)

// obfuscationKeyFile is the sensitive companion of an obfuscated backup: the
// secret behind the token hashes and the reverse mapping
type obfuscationKeyFile struct {
	Version int               `json:"version"`
	Secret  string            `json:"secret"`
	Map     map[string]string `json:"map"`
}

// obfuscator replaces identifying tokens with keyed hashes. The same secret
// and token always produce the same hash, so relations between subjects,
// filters and names survive.
//
// The hash is HMAC-SHA-256 under a random 32-byte secret. Tokens are
// low-entropy words like "orders", so an unkeyed hash is reversed by hashing
// a wordlist. With the secret, recovery needs the key file. A random token
// per original would obfuscate too, but its output depends on RNG and
// encounter order, and an edit guarantees byte-identical output for the same
// source, options and key file. The secret alone also keeps separate backups
// consistent: two edits sharing a key file hash a common token identically
// even when neither saw the other's map, and loading a key file re-verifies
// every mapping against the secret, so a tampered or mismatched file is
// rejected instead of producing conflicting tokens.
//
// Truncating to 10 bytes and encoding lowercase base32-hex gives a
// 16-character token that is legal everywhere an original can appear: a
// subject token, a KV key segment, a stream or consumer name. Stream names
// become directories in the file store, so mixed case would collide on
// case-insensitive filesystems, and base64 uses '/' and '+'. Hex needs 20
// characters for the same bytes, and the subject is rewritten in every
// message record, so token length inflates the archive. A collision within
// 80 bits needs around a trillion distinct tokens, and token() fails on one
// anyway instead of silently merging two originals
type obfuscator struct {
	secret  []byte
	forward map[string]string
	reverse map[string]string
	bodies  uint64
}

func newObfuscator(keyFile string) (*obfuscator, error) {
	o := &obfuscator{forward: map[string]string{}, reverse: map[string]string{}}
	if keyFile == "" {
		o.secret = make([]byte, secretBytes)
		if _, err := rand.Read(o.secret); err != nil {
			return nil, err
		}
		return o, nil
	}

	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	var kf obfuscationKeyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("%s: %w", keyFile, err)
	}
	if kf.Version != keyFileVersion {
		return nil, fmt.Errorf("%s: unsupported key file version %d", keyFile, kf.Version)
	}
	o.secret, err = base64.StdEncoding.DecodeString(kf.Secret)
	if err != nil || len(o.secret) != secretBytes {
		return nil, fmt.Errorf("%s: invalid secret", keyFile)
	}
	for hashed, original := range kf.Map {
		if o.hash(original) != hashed {
			return nil, fmt.Errorf("%s: mapping for %q does not match the secret", keyFile, hashed)
		}
		o.forward[original] = hashed
		o.reverse[hashed] = original
	}

	return o, nil
}

func (o *obfuscator) hash(tok string) string {
	mac := hmac.New(sha256.New, o.secret)
	mac.Write([]byte(tok))
	return strings.ToLower(tokenEncoding.EncodeToString(mac.Sum(nil)[:tokenBytes]))
}

func (o *obfuscator) token(tok string) (string, error) {
	if tok == "" {
		return tok, nil
	}
	if h, ok := o.forward[tok]; ok {
		return h, nil
	}
	h := o.hash(tok)
	if other, taken := o.reverse[h]; taken && other != tok {
		return "", fmt.Errorf("obfuscation hash collision between %q and %q", tok, other)
	}
	o.forward[tok] = h
	o.reverse[h] = tok
	return h, nil
}

// subject hashes a subject token by token; wildcards and the $KV and $O
// roots pass through so filters keep their relations to the hashed subjects
func (o *obfuscator) subject(subj string) (string, error) {
	return o.rewriteSubject(subj, false)
}

// destination additionally passes {{...}} mapping functions through; only
// republish and subject transform destinations carry them, anywhere else a
// {{...}} token is literal subject text and subject hashes it
func (o *obfuscator) destination(subj string) (string, error) {
	return o.rewriteSubject(subj, true)
}

func (o *obfuscator) rewriteSubject(subj string, mappingFuncs bool) (string, error) {
	if subj == "" {
		return subj, nil
	}
	toks := strings.Split(subj, ".")
	for i, tok := range toks {
		switch {
		case tok == "*", tok == ">", tok == kvSubjectRoot, tok == objSubjectRoot:
		case mappingFuncs && strings.HasPrefix(tok, "{{") && strings.HasSuffix(tok, "}}"):
		default:
			h, err := o.token(tok)
			if err != nil {
				return "", err
			}
			toks[i] = h
		}
	}
	return strings.Join(toks, "."), nil
}

func (o *obfuscator) subjects(subjs []string) ([]string, error) {
	if subjs == nil {
		return nil, nil
	}
	out := make([]string, len(subjs))
	for i, s := range subjs {
		h, err := o.subject(s)
		if err != nil {
			return nil, err
		}
		out[i] = h
	}
	return out, nil
}

func (o *obfuscator) streamName(name string) (string, error) {
	for _, prefix := range []string{kvStreamPrefix, objStorePrefix} {
		if rest, ok := strings.CutPrefix(name, prefix); ok && rest != "" {
			h, err := o.token(rest)
			return prefix + h, err
		}
	}
	return o.token(name)
}

func (o *obfuscator) transforms(ts []api.SubjectTransformConfig) ([]api.SubjectTransformConfig, error) {
	if ts == nil {
		return nil, nil
	}
	out := make([]api.SubjectTransformConfig, len(ts))
	for i, t := range ts {
		var err error
		if out[i].Source, err = o.subject(t.Source); err != nil {
			return nil, err
		}
		if out[i].Destination, err = o.destination(t.Destination); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (o *obfuscator) streamSource(src *api.StreamSource) (*api.StreamSource, error) {
	if src == nil {
		return nil, nil
	}
	out := *src
	var err error
	if out.Name, err = o.streamName(src.Name); err != nil {
		return nil, err
	}
	if out.FilterSubject, err = o.subject(src.FilterSubject); err != nil {
		return nil, err
	}
	if out.SubjectTransforms, err = o.transforms(src.SubjectTransforms); err != nil {
		return nil, err
	}
	if src.External != nil {
		ext := *src.External
		if ext.ApiPrefix, err = o.subject(src.External.ApiPrefix); err != nil {
			return nil, err
		}
		if ext.DeliverPrefix, err = o.subject(src.External.DeliverPrefix); err != nil {
			return nil, err
		}
		out.External = &ext
	}
	if src.Consumer != nil {
		c := *src.Consumer
		if c.Name, err = o.token(src.Consumer.Name); err != nil {
			return nil, err
		}
		if c.DeliverSubject, err = o.subject(src.Consumer.DeliverSubject); err != nil {
			return nil, err
		}
		out.Consumer = &c
	}
	return &out, nil
}

// config rewrites the identifying fields of a stream configuration and
// drops the descriptive ones; limits and retention stay verbatim
func (o *obfuscator) config(cfg api.StreamConfig) (api.StreamConfig, error) {
	out := cfg
	var err error
	if out.Name, err = o.streamName(cfg.Name); err != nil {
		return out, err
	}
	if out.Subjects, err = o.subjects(cfg.Subjects); err != nil {
		return out, err
	}
	if out.Mirror, err = o.streamSource(cfg.Mirror); err != nil {
		return out, err
	}
	if cfg.Sources != nil {
		out.Sources = make([]*api.StreamSource, len(cfg.Sources))
		for i, src := range cfg.Sources {
			if out.Sources[i], err = o.streamSource(src); err != nil {
				return out, err
			}
		}
	}
	if cfg.SubjectTransform != nil {
		ts, err := o.transforms([]api.SubjectTransformConfig{*cfg.SubjectTransform})
		if err != nil {
			return out, err
		}
		out.SubjectTransform = &ts[0]
	}
	if cfg.RePublish != nil {
		rp := *cfg.RePublish
		if rp.Source, err = o.subject(cfg.RePublish.Source); err != nil {
			return out, err
		}
		if rp.Destination, err = o.destination(cfg.RePublish.Destination); err != nil {
			return out, err
		}
		out.RePublish = &rp
	}
	out.Description = ""
	out.Metadata = nil
	out.Placement = nil
	return out, nil
}

// consumer rewrites a consumer entry through the server's own snapshot type
// so every field the restore reads is covered; state stays verbatim
func (o *obfuscator) consumer(name string, data []byte) (string, []byte, error) {
	var scs server.SnapshotConsumerState
	if err := json.Unmarshal(data, &scs); err != nil {
		return "", nil, fmt.Errorf("consumer %q: %w", name, err)
	}
	cfg := *scs.ConsumerConfig
	var err error
	if cfg.Durable, err = o.token(cfg.Durable); err != nil {
		return "", nil, err
	}
	if cfg.Name, err = o.token(cfg.Name); err != nil {
		return "", nil, err
	}
	if cfg.FilterSubject, err = o.subject(cfg.FilterSubject); err != nil {
		return "", nil, err
	}
	if cfg.FilterSubjects, err = o.subjects(cfg.FilterSubjects); err != nil {
		return "", nil, err
	}
	if cfg.DeliverSubject, err = o.subject(cfg.DeliverSubject); err != nil {
		return "", nil, err
	}
	if cfg.DeliverGroup, err = o.token(cfg.DeliverGroup); err != nil {
		return "", nil, err
	}
	cfg.PriorityGroups = slices.Clone(cfg.PriorityGroups)
	for i, g := range cfg.PriorityGroups {
		if cfg.PriorityGroups[i], err = o.token(g); err != nil {
			return "", nil, err
		}
	}
	cfg.Description = ""
	cfg.Metadata = nil
	scs.ConsumerConfig = &cfg

	hashedName, err := o.token(name)
	if err != nil {
		return "", nil, err
	}
	out, err := json.Marshal(scs)
	if err != nil {
		return "", nil, err
	}
	return hashedName, out, nil
}

// message rewrites a message's subject and headers; control headers stay
// verbatim, subject-valued headers are token hashed and every other value
// is hashed whole. Headers are written in key order so the output is
// deterministic. The body is dropped by the caller
func (o *obfuscator) message(subject string, h nats.Header) (string, []byte, error) {
	subj, err := o.subject(subject)
	if err != nil {
		return "", nil, err
	}
	if len(h) == 0 {
		return subj, nil, nil
	}

	var out bytes.Buffer
	out.WriteString("NATS/1.0\r\n")
	for _, key := range slices.Sorted(maps.Keys(h)) {
		for _, val := range h[key] {
			v, err := o.headerValue(key, val)
			if err != nil {
				return "", nil, err
			}
			out.WriteString(key + ": " + v + "\r\n")
		}
	}
	out.WriteString("\r\n")
	return subj, out.Bytes(), nil
}

func (o *obfuscator) headerValue(key, val string) (string, error) {
	switch {
	case equalsAny(key, server.KVOperation, server.JSMarkerReason, server.JSMessageTTL, server.JSMsgRollup, server.JSExpectedLastSubjSeq,
		server.JSSchedulePattern, server.JSScheduleTTL, server.JSScheduleTimeZone):
		return val, nil
	case equalsAny(key, server.JSExpectedLastSubjSeqSubj, server.JSScheduleTarget, server.JSScheduleSource):
		return o.subject(val)
	default:
		return o.token(val)
	}
}

func equalsAny(key string, names ...string) bool {
	for _, n := range names {
		if strings.EqualFold(key, n) {
			return true
		}
	}
	return false
}

func (o *obfuscator) keyFileBytes() ([]byte, error) {
	return json.MarshalIndent(obfuscationKeyFile{
		Version: keyFileVersion,
		Secret:  base64.StdEncoding.EncodeToString(o.secret),
		Map:     o.reverse,
	}, "", "  ")
}

// keyFilePath is the sibling of the target that receives the mappings
func keyFilePath(target string) string {
	return filepath.Clean(target) + keyFileSuffix
}

// writeKeyFile refuses to clobber a key file it did not read this run and
// replaces the file atomically, so the input key file is never left
// truncated. It reports whether the file is new
func (o *obfuscator) writeKeyFile(path string, inputKeyFile string) (created bool, err error) {
	same := false
	if inputKeyFile != "" {
		a, errA := filepath.Abs(inputKeyFile)
		b, errB := filepath.Abs(path)
		same = errA == nil && errB == nil && a == b
	}
	if !same {
		if _, err := os.Stat(path); err == nil {
			return false, fmt.Errorf("key file %s already exists", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}

	data, err := o.keyFileBytes()
	if err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return false, err
	}
	return !same, nil
}
