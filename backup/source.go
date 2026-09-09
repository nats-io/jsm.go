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
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
}

func openSource(dir string) (*source, error) {
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

	src := &source{dataPath: data, metaFile: mf, f: f, stat: stat, digest: sha256.New()}
	src.tee = io.TeeReader(f, src.digest)

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
	return s.f, nil
}

func (s *source) isKVBucket() bool {
	cfg := s.metaFile.Config
	if !jsm.IsKVBucketStream(cfg.Name) {
		return false
	}
	bucket := strings.TrimPrefix(cfg.Name, kvStreamPrefix)
	return bucket != "" && len(cfg.Subjects) == 1 && cfg.Subjects[0] == kvSubjectPrefix+bucket+".>"
}

// target is the output directory, built in a sibling staging directory and
// moved into place with one rename once the sentinel and meta file are on disk
type target struct {
	final   string
	staging string
	existed bool
}

func prepareTarget(dir string) (*target, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	t := &target{final: abs}
	st, err := os.Stat(abs)
	switch {
	case err == nil:
		if !st.IsDir() {
			return nil, fmt.Errorf("target %s exists and is not a directory", abs)
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			return nil, fmt.Errorf("target directory %s is not empty", abs)
		}
		t.existed = true
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	t.staging, err = os.MkdirTemp(filepath.Dir(abs), "."+filepath.Base(abs)+".editing-")
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *target) path(name string) string {
	return filepath.Join(t.staging, name)
}

func (t *target) writeFile(name string, data []byte) error {
	f, err := os.OpenFile(t.path(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// commit renames the staging directory onto the target; renaming over an
// existing empty directory is atomic where the platform allows it, otherwise
// the target is removed first
func (t *target) commit() error {
	// Windows refuses to sync a directory handle and NTFS journals the
	// entries anyway, so the directory sync only runs elsewhere
	if runtime.GOOS != "windows" {
		d, err := os.Open(t.staging)
		if err != nil {
			return err
		}
		if err := d.Sync(); err != nil {
			d.Close()
			return err
		}
		d.Close()
	}

	if err := os.Rename(t.staging, t.final); err == nil || !t.existed {
		return err
	}
	if err := os.Remove(t.final); err != nil {
		return err
	}
	return os.Rename(t.staging, t.final)
}

func (t *target) discard() {
	os.RemoveAll(t.staging)
}
