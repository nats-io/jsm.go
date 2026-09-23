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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"github.com/nats-io/jsm.go"
)

const (
	kvStreamPrefix  = "KV_"
	kvSubjectPrefix = "$KV."
)

// source is a backup directory opened for editing; the archive handle is held
// for the whole edit so a second pass reads exactly what the first observed
type source struct {
	dataPath string
	metaFile *metaFile
	f        *os.File
	stat     os.FileInfo
	digest   hash.Hash
	tee      io.Reader
	prog     *progress
}

func openSource(dir string, prog *progress) (*source, error) {
	data, meta, err := backupPaths(dir)
	if err != nil {
		return nil, err
	}

	mf, err := readMetaFile(meta)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(data)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	prog.total = uint64(stat.Size())
	src := &source{dataPath: data, metaFile: mf, f: f, stat: stat, digest: sha256.New(), prog: prog}
	src.tee = io.TeeReader(prog.reader(f), src.digest)

	return src, nil
}

func (s *source) close() error {
	return s.f.Close()
}

// finishDigest hashes whatever the decoder left unread so the digest covers the whole file
func (s *source) finishDigest() (string, error) {
	if _, err := io.Copy(io.Discard, s.tee); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(s.digest.Sum(nil)), nil
}

// rewind positions the held handle at the start for a second pass after
// checking the file was not modified in place since it was opened
func (s *source) rewind() (io.Reader, error) {
	now, err := s.f.Stat()
	if err != nil {
		return nil, err
	}
	if now.Size() != s.stat.Size() || !now.ModTime().Equal(s.stat.ModTime()) {
		return nil, fmt.Errorf("%s changed during the edit (size %d -> %d, modified %s -> %s)", s.dataPath, s.stat.Size(), now.Size(), s.stat.ModTime().Format("15:04:05.000"), now.ModTime().Format("15:04:05.000"))
	}
	if _, err := s.f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	s.prog.nextPass()
	return s.prog.reader(s.f), nil
}

func (s *source) isKVBucket() bool {
	cfg := s.metaFile.Config
	if !jsm.IsKVBucketStream(cfg.Name) {
		return false
	}
	bucket := strings.TrimPrefix(cfg.Name, kvStreamPrefix)
	return bucket != "" && len(cfg.Subjects) == 1 && cfg.Subjects[0] == kvSubjectPrefix+bucket+".>"
}
