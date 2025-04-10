package archive

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"time"
)

// pagedWriter writes JSON entries to paged files inside the zip archive.
type pagedWriter struct {
	zipWriter *zip.Writer
	dir       string
	buf       *bytes.Buffer
	pageIndex int
	ts        *time.Time
}

func newPagedWriter(z *zip.Writer, dir string, ts *time.Time) *pagedWriter {
	return &pagedWriter{
		zipWriter: z,
		dir:       dir,
		buf:       &bytes.Buffer{},
		pageIndex: 1,
		ts:        ts,
	}
}

func (pw *pagedWriter) WriteEntry(r io.Reader) error {
	filename := filepath.Join(pw.dir, fmt.Sprintf("%04d.json", pw.pageIndex))
	header := &zip.FileHeader{
		Name:     filename,
		Method:   zip.Deflate,
		Modified: tsToUTC(pw.ts),
	}

	w, err := pw.zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	if _, err := io.Copy(w, r); err != nil {
		return fmt.Errorf("failed to write zip entry: %w", err)
	}

	pw.pageIndex++
	return nil
}

func tsToUTC(ts *time.Time) time.Time {
	if ts != nil {
		return ts.UTC()
	}
	return time.Now().UTC()
}
