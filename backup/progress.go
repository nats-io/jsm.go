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
	"io"
	"time"
)

// Progress is what an edit or a scan reports while it reads the archive.
// The archive size is known before the first byte is read, so BytesRead
// over BytesTotal is exact
type Progress interface {
	// StartTime is when the operation started
	StartTime() time.Time
	// BytesRead is the compressed archive bytes consumed in the current pass
	BytesRead() uint64
	// BytesTotal is the compressed archive size on disk
	BytesTotal() uint64
	// Entries is the archive entries decoded in the current pass
	Entries() uint64
	// Pass is the pass being read, counted from 1
	Pass() int
	// Passes is how many passes the operation reads, a per-subject edit reads two
	Passes() int
	// Finished is true once the last pass has been read
	Finished() bool
}

type progress struct {
	start    time.Time
	read     uint64
	total    uint64
	entries  uint64
	pass     int
	passes   int
	finished bool
	cb       func(Progress)
}

func newProgress(cb func(Progress), passes int) *progress {
	return &progress{start: time.Now(), pass: 1, passes: passes, cb: cb}
}

func (p *progress) StartTime() time.Time { return p.start }
func (p *progress) BytesRead() uint64    { return p.read }
func (p *progress) BytesTotal() uint64   { return p.total }
func (p *progress) Entries() uint64      { return p.entries }
func (p *progress) Pass() int            { return p.pass }
func (p *progress) Passes() int          { return p.passes }
func (p *progress) Finished() bool       { return p.finished }

func (p *progress) notify() {
	if p.cb != nil {
		p.cb(p)
	}
}

func (p *progress) nextPass() {
	p.pass++
	p.read = 0
	p.entries = 0
}

func (p *progress) finish() {
	p.finished = true
	p.notify()
}

// reader counts the archive bytes handed to the decoder and reports after
// every read, which is once per compressed block
func (p *progress) reader(r io.Reader) io.Reader {
	return &countingReader{r: r, p: p}
}

type countingReader struct {
	r io.Reader
	p *progress
}

func (c *countingReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.p.read += uint64(n)
	c.p.notify()
	return n, err
}
