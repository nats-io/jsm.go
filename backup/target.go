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
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// target is an output backup directory, built in a sibling staging directory
// and renamed into place once the sentinel and meta file are on disk, so an
// interrupted writer never leaves a directory that verifies
type target struct {
	final   string
	staging string
	existed bool
}

// newTarget prepares dir, which must not exist or must be an empty directory
func newTarget(dir string) (*target, error) {
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

	t.staging, err = os.MkdirTemp(filepath.Dir(abs), "."+filepath.Base(abs)+".backup-")
	if err != nil {
		return nil, err
	}

	return t, nil
}

// archive opens the archive file in staging, buffered and synced on Close
func (t *target) archive() (*syncedFile, error) {
	f, err := os.OpenFile(filepath.Join(t.staging, DataFile), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	return &syncedFile{Writer: bufio.NewWriterSize(f, 1<<20), f: f}, nil
}

type syncedFile struct {
	*bufio.Writer
	f *os.File
}

func (s *syncedFile) Close() error {
	if err := s.Writer.Flush(); err != nil {
		s.f.Close()
		return err
	}
	if err := s.f.Sync(); err != nil {
		s.f.Close()
		return err
	}
	return s.f.Close()
}

// patch overwrites bytes already written
func (s *syncedFile) patch(p []byte, off int64) error {
	if err := s.Writer.Flush(); err != nil {
		return err
	}
	_, err := s.f.WriteAt(p, off)
	return err
}

// writeMeta writes the meta file into staging, synced
func (t *target) writeMeta(mf *metaFile) error {
	data, err := mf.marshal()
	if err != nil {
		return err
	}
	return t.writeFile(MetaFile, data)
}

func (t *target) writeFile(name string, data []byte) error {
	f, err := os.OpenFile(filepath.Join(t.staging, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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

// commit renames the staging directory onto the target. Renaming over an
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

// discard removes the staging directory
func (t *target) discard() {
	os.RemoveAll(t.staging)
}
