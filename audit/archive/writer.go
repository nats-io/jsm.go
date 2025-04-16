// Copyright 2024 The NATS Authors
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

package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Writer encapsulates a zip writer for the underlying archive file, but also tracks metadata used by the Reader to
// construct indices
type Writer struct {
	path         string
	fileWriter   *os.File
	zipWriter    *zip.Writer
	manifestMap  map[string][]*Tag
	ts           *time.Time
	pagedWriters map[string]*pagedWriter
}

// Close closes the writer
func (w *Writer) Close() error {
	// Add manifest file to archive before closing it
	if w.zipWriter != nil && w.fileWriter != nil {
		err := w.Add(w.manifestMap, internalTagManifest())
		if err != nil {
			return fmt.Errorf("failed to add manifest: %w", err)
		}
	}

	// Close and null the zip writer
	if w.zipWriter != nil {
		err := w.zipWriter.Close()
		w.zipWriter = nil
		if err != nil {
			return fmt.Errorf("failed to close archive zip writer: %w", err)
		}
	}

	// Close and null the file writer
	if w.fileWriter != nil {
		err := w.fileWriter.Close()
		w.fileWriter = nil
		if err != nil {
			return fmt.Errorf("failed to close archive file writer: %w", err)
		}
	}

	return nil
}

// addArtifact low-level API that adds bytes without adding to the index, used for special files
func (w *Writer) addArtifact(name string, content *bytes.Reader) error {
	f, err := w.zipWriter.Create(name)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, content)
	if err != nil {
		return err
	}

	return nil
}

// Add serializes the given artifact to JSON and adds it to the archive, it creates a file name based on the provided
// tags and ensures uniqueness. The artifact is also added to the manifest for indexing, enabling tag-based querying
// in via Reader
func (w *Writer) Add(artifact any, tags ...*Tag) error {
	// Encode the artifact as (pretty-formatted) JSON
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(artifact)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}
	return w.AddRaw(&buf, "json", tags...)
}

// SetTime sets the timestamp files in the archive should have, otherwise current time is used
func (w *Writer) SetTime(t time.Time) {
	w.ts = &t
}

// AddRaw adds the given artifact to the archive similarly to Add.
// The artifact is assumed to be already serialized and is copied as-is byte for byte.
// If the artifact is tagged as "special", it will be written as a single non-paged file.
func (w *Writer) AddRaw(reader io.Reader, extension string, tags ...*Tag) error {
	if w.zipWriter == nil {
		return fmt.Errorf("attempting to write into a closed writer")
	}

	dir, err := dirNameFromTags(tags)
	if err != nil {
		return fmt.Errorf("failed to determine directory from tags: %w", err)
	}

	// Special artifacts should not use paging
	if isNonPagedArtifact(tags) {
		filename, err := createFilenameFromTags(extension, tags)
		if err != nil {
			return fmt.Errorf("unable to create filename for file with tags %+v", tags)
		}

		header := &zip.FileHeader{
			Name:     filename,
			Method:   zip.Deflate,
			Modified: tsToUTC(w.ts),
		}
		wr, err := w.zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create zip entry for special artifact: %w", err)
		}

		if _, err := io.Copy(wr, reader); err != nil {
			return fmt.Errorf("failed to write special artifact: %w", err)
		}

		if err := w.addPathToManifest(0, extension, tags); err != nil {
			return err
		}

		return nil
	}

	// Everything else gets paged
	pw := w.PagedWriter(dir)

	if err := pw.WriteEntry(reader); err != nil {
		return fmt.Errorf("failed to write page: %w", err)
	}

	// subtract 1 to get correct page just written since WriteEntry increments this value
	return w.addPathToManifest(pw.pageIndex-1, extension, tags)
}

func isNonPagedArtifact(tags []*Tag) bool {
	for _, t := range tags {
		if t.Name == specialTagLabel {
			return true
		}
		if t.Name == typeTagLabel && t.Value == profileArtifactType {
			return true
		}
	}
	return false
}

// NewWriter creates a new writer for the file at the given archivePath.
// Writer creates a ZIP file whose content has additional structure and metadata.
// If archivePath is an existing file, it will be overwritten.
func NewWriter(archivePath string) (*Writer, error) {
	fileWriter, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}

	zipWriter := zip.NewWriter(fileWriter)

	return &Writer{
		path:         archivePath,
		fileWriter:   fileWriter,
		zipWriter:    zipWriter,
		manifestMap:  make(map[string][]*Tag),
		pagedWriters: make(map[string]*pagedWriter),
	}, nil
}

func (w *Writer) PagedWriter(dir string) *pagedWriter {
	if pw, ok := w.pagedWriters[dir]; ok {
		return pw
	}
	pw := newPagedWriter(w.zipWriter, dir, w.ts)
	w.pagedWriters[dir] = pw
	return pw
}

func (w *Writer) addPathToManifest(pageIndex int, extension string, tags []*Tag) error {
	var path string
	var err error

	if isNonPagedArtifact(tags) {
		if pageIndex != 0 {
			// This is an extreme edge case, and if it happens we broke the code
			return fmt.Errorf("non-paged artifact with non-zero page index: %d", pageIndex)
		}

		// Use full filename for non-paged artifacts
		path, err = createFilenameFromTags(extension, tags)
		if err != nil {
			return fmt.Errorf("failed to create filename from tags: %w", err)
		}
	} else {
		// Use numbered page for everything else
		dir, err := dirNameFromTags(tags)
		if err != nil {
			return fmt.Errorf("failed to create manifest path from tags: %w", err)
		}
		pageName := fmt.Sprintf("%04d.%s", pageIndex, extension)
		path = filepath.Join(dir, pageName)
	}

	w.manifestMap[path] = tags
	return nil
}
