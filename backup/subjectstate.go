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
	"hash/maphash"
	"unsafe"
)

// subjectCounter counts distinct subjects through 64-bit hashes, so the
// count costs a few bytes per subject rather than the subject itself
type subjectCounter struct {
	seed maphash.Seed
	seen map[uint64]struct{}
}

func newSubjectCounter() *subjectCounter {
	return &subjectCounter{seed: maphash.MakeSeed(), seen: map[uint64]struct{}{}}
}

func (c *subjectCounter) add(subject string) {
	c.seen[maphash.String(c.seed, subject)] = struct{}{}
}

func (c *subjectCounter) count() int {
	return len(c.seen)
}

// subjectState decides which messages of the filtered result survive a
// per-subject edit. Messages are observed in ascending sequence order; after
// Resolve, Keeps answers for any observed message. Nothing outside this file
// knows how the state is held
type subjectState interface {
	Observe(subject string, seq uint64, size uint64, tomb bool)
	Resolve() resolved
	Keeps(subject string, seq uint64) bool
	Stats() (keys uint64, bytes uint64)
}

// resolved is what a per-subject edit will write: exact message, byte and
// subject totals and the lowest surviving sequence
type resolved struct {
	msgs     uint64
	bytes    uint64
	subjects uint64
	firstSeq uint64
}

func (r *resolved) add(seq, size uint64) {
	r.msgs++
	r.bytes += size
	if r.firstSeq == 0 || seq < r.firstSeq {
		r.firstSeq = seq
	}
}

func newSubjectState(o *editOptions) subjectState {
	if o.kvCompact {
		return &kvCompactState{slots: map[string]kvSlot{}}
	}
	return &lastPerSubjectState{n: o.lastPerSubject, rings: map[string]ring{}}
}

// kvCompactState keeps the latest revision per key and drops keys whose
// latest revision is a tombstone
type kvCompactState struct {
	slots map[string]kvSlot
	keyB  uint64
}

type kvSlot struct {
	seq  uint64
	size uint64
	tomb bool
}

func (s *kvCompactState) Observe(subject string, seq uint64, size uint64, tomb bool) {
	if _, seen := s.slots[subject]; !seen {
		s.keyB += uint64(len(subject))
	}
	s.slots[subject] = kvSlot{seq: seq, size: size, tomb: tomb}
}

func (s *kvCompactState) Resolve() resolved {
	var res resolved
	for _, slot := range s.slots {
		if !slot.tomb {
			res.add(slot.seq, slot.size)
			res.subjects++
		}
	}
	return res
}

func (s *kvCompactState) Keeps(subject string, seq uint64) bool {
	slot, ok := s.slots[subject]
	return ok && !slot.tomb && slot.seq == seq
}

func (s *kvCompactState) Stats() (uint64, uint64) {
	return uint64(len(s.slots)), s.keyB + uint64(len(s.slots))*uint64(unsafe.Sizeof(kvSlot{}))
}

// lastPerSubjectState keeps the newest n messages of every subject in a
// ring per subject; sequences arrive ascending, so once a ring is full the
// cursor always points at its oldest entry
type lastPerSubjectState struct {
	n     int
	rings map[string]ring
	keyB  uint64
	slots uint64
}

type ring struct {
	entries []ringEntry
	next    int
}

type ringEntry struct {
	seq  uint64
	size uint64
}

func (s *lastPerSubjectState) Observe(subject string, seq uint64, size uint64, _ bool) {
	r, seen := s.rings[subject]
	if !seen {
		s.keyB += uint64(len(subject))
	}
	e := ringEntry{seq: seq, size: size}
	if len(r.entries) < s.n {
		r.entries = append(r.entries, e)
		s.slots++
	} else {
		r.entries[r.next] = e
		r.next = (r.next + 1) % s.n
	}
	s.rings[subject] = r
}

func (s *lastPerSubjectState) Resolve() resolved {
	var res resolved
	for _, r := range s.rings {
		for _, e := range r.entries {
			res.add(e.seq, e.size)
		}
	}
	res.subjects = uint64(len(s.rings))
	return res
}

func (s *lastPerSubjectState) Keeps(subject string, seq uint64) bool {
	for _, e := range s.rings[subject].entries {
		if e.seq == seq {
			return true
		}
	}
	return false
}

func (s *lastPerSubjectState) Stats() (uint64, uint64) {
	return uint64(len(s.rings)), s.keyB + s.slots*uint64(unsafe.Sizeof(ringEntry{}))
}
