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
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nats-io/jsm.go/api"
)

// Result describes a finished edit: the configuration and state written to
// the meta file and a Report of what the edit did
type Result struct {
	Config api.StreamConfig
	State  api.StreamState
	Report Report
}

// DropCounts records how many messages each filter removed, a message is
// attributed to the first filter that rejected it, evaluated in the order
// sequence, time, subject, header, payload, then last-per-subject or kv-compact
type DropCounts struct {
	Sequence       uint64
	Time           uint64
	Subject        uint64
	Header         uint64
	Payload        uint64
	LastPerSubject uint64
	KVCompact      uint64
}

// Total is the number of messages dropped by any filter
func (d DropCounts) Total() uint64 {
	return d.Sequence + d.Time + d.Subject + d.Header + d.Payload + d.LastPerSubject + d.KVCompact
}

func (d *DropCounts) add(kind dropKind) {
	switch kind {
	case dropSequence:
		d.Sequence++
	case dropTime:
		d.Time++
	case dropSubject:
		d.Subject++
	case dropHeader:
		d.Header++
	case dropPayload:
		d.Payload++
	case dropLastPerSubject:
		d.LastPerSubject++
	case dropKVCompact:
		d.KVCompact++
	}
}

// Report is what an edit did; it is outside the determinism guarantee since
// the MaxAge warning uses the edit-time clock
type Report struct {
	SourceMessages   uint64
	Kept             uint64
	SourceSubjects   int
	KeptSubjects     int
	Dropped          DropCounts
	ConsumersKept    int
	ConsumersDropped int
	// TombstonesRemovedByContentFilters counts KV delete, purge and limit
	// markers that a header or payload filter dropped
	TombstonesRemovedByContentFilters uint64
	// SubjectStateKeys and SubjectStateBytes describe the per-subject state a
	// two-pass edit held: distinct subjects and the accounted size of their
	// keys and slots, not process memory
	SubjectStateKeys  uint64
	SubjectStateBytes uint64
	// Warnings describe stream limits that will trim the result on restore
	Warnings []string

	Obfuscation *ObfuscationReport
}

// ObfuscationReport describes an obfuscated edit
type ObfuscationReport struct {
	TokensMapped int
	BodiesPadded uint64
	KeyFile      string
}

// Edit reads the backup in srcDir, applies the options and writes a new
// backup to dstDir, which must not exist or be an empty directory. The
// output appears atomically: it is staged beside dstDir and renamed into
// place only once the sentinel and meta file are on disk
func Edit(ctx context.Context, srcDir string, dstDir string, opts ...EditOption) (*Result, error) {
	o := &editOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if err := o.validate(); err != nil {
		return nil, err
	}
	passes := 1
	if o.perSubject() && (!o.dryRun || o.obfuscate) {
		passes = 2
	}
	prog := newProgress(o.notify, passes)
	src, err := openSource(srcDir, prog)
	if err != nil {
		return nil, err
	}
	defer src.close()

	if o.kvCompact && !src.isKVBucket() {
		return nil, fmt.Errorf("KVCompact requires a KV bucket backup: stream %q with subjects %v is not one", src.metaFile.Config.Name, src.metaFile.Config.Subjects)
	}

	ed := &editor{ctx: ctx, o: o, src: src, cfg: src.metaFile.Config, now: time.Now(), srcSubjects: newSubjectCounter()}
	if !o.perSubject() {
		ed.keptSubjects = newSubjectCounter()
	}
	if o.obfuscate {
		if ed.obf, err = newObfuscator(o.keyFile); err != nil {
			return nil, err
		}
	}

	var tgt *target
	if !o.dryRun {
		tgt, err = newTarget(dstDir)
		if err != nil {
			return nil, err
		}
		defer func() {
			if tgt != nil {
				tgt.discard()
			}
		}()
	}

	if err := ed.run(tgt); err != nil {
		return nil, err
	}
	prog.finish()

	result, err := ed.result()
	if err != nil {
		return nil, err
	}
	if tgt == nil {
		return result, nil
	}

	source, err := ed.sourceInfo()
	if err != nil {
		return nil, err
	}
	mf := &metaFile{
		Config: result.Config,
		State:  result.State,
		Edit: &EditInfo{
			Version:      o.toolVersion,
			Options:      o.canonical(),
			SourceDigest: ed.digest,
			Obfuscated:   o.obfuscate,
		},
		Source: source,
	}
	if err := tgt.writeMeta(mf); err != nil {
		return nil, err
	}

	if ed.obf != nil {
		kf := keyFilePath(tgt.final)
		created, err := ed.obf.writeKeyFile(kf, o.keyFile)
		if err != nil {
			return nil, err
		}
		if err := tgt.commit(); err != nil {
			if created {
				os.Remove(kf)
			}
			return nil, err
		}
		result.Report.Obfuscation.KeyFile = kf
	} else if err := tgt.commit(); err != nil {
		return nil, err
	}
	tgt = nil

	return result, nil
}

type editor struct {
	ctx      context.Context
	o        *editOptions
	src      *source
	cfg      api.StreamConfig
	now      time.Time
	enc      *Encoder
	out      *syncedFile
	headTs   int64
	obf      *obfuscator
	srcState api.StreamState
	firstOut uint64
	bytes    uint64
	overAge  uint64
	digest   string
	report   Report

	// srcSubjects sees every source message; keptSubjects sees every written
	// one on the single pass, a per-subject edit takes its kept count from
	// Resolve instead so a dry run that stops there still reports it
	srcSubjects      *subjectCounter
	keptSubjects     *subjectCounter
	keptSubjectCount int
}

func (e *editor) run(tgt *target) error {
	pass := e.singlePass
	if e.o.perSubject() {
		pass = e.twoPass
	}
	if tgt == nil {
		return pass()
	}

	out, err := tgt.archive()
	if err != nil {
		return err
	}
	e.out = out
	e.enc = newEntryEncoder(out)

	err = pass()
	if cerr := e.enc.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		var head []byte
		head, err = encodeStateHead(e.headTs, e.archiveState(&api.StreamState{Msgs: e.report.Kept, Bytes: e.bytes}))
		if err == nil {
			err = out.patch(head, 0)
		}
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

func (e *editor) singlePass() error {
	dec := NewDecoder(e.src.tee)
	for {
		if err := e.ctx.Err(); err != nil {
			return err
		}

		item, err := dec.Next()
		if err != nil {
			return err
		}
		e.src.prog.entries++

		switch it := item.(type) {
		case *State:
			e.srcState = it.State
			if err := e.writeState(it.Ts); err != nil {
				return err
			}
		case *Consumer:
			if err := e.consumer(it); err != nil {
				return err
			}
		case *Message:
			if err := e.filterMessage(it); err != nil {
				return err
			}
		case End:
			if e.enc != nil {
				if err := e.enc.WriteEnd(); err != nil {
					return err
				}
			}
			e.digest, err = e.src.finishDigest()
			return err
		}
	}
}

// writeState reserves the leading state.json, run overwrites it with the
// totals written once the pass ends
func (e *editor) writeState(ts int64) error {
	if e.out == nil {
		return nil
	}
	e.headTs = ts
	head, err := encodeStateHead(ts, e.archiveState(nil))
	if err != nil {
		return err
	}
	_, err = e.out.Write(head)
	return err
}

// archiveState is the state.json the output archive carries, exact holds the
// message and byte totals once they are known
func (e *editor) archiveState(exact *api.StreamState) api.StreamState {
	st := api.StreamState{
		FirstSeq:  e.srcState.FirstSeq,
		LastSeq:   e.srcLastSeq(),
		Consumers: e.srcState.Consumers,
	}
	if e.o.renumber {
		st.FirstSeq = 1
		st.LastSeq = 0
		st.Consumers = 0
	}
	if exact != nil {
		st.Msgs = exact.Msgs
		st.Bytes = exact.Bytes
		if e.o.renumber {
			st.LastSeq = exact.Msgs
		}
	}
	return st
}

func (e *editor) consumer(c *Consumer) error {
	if e.o.renumber {
		e.report.ConsumersDropped++
		return nil
	}
	e.report.ConsumersKept++
	return e.writeConsumer(c)
}

func (e *editor) writeConsumer(c *Consumer) error {
	name, data := c.Name, c.Data
	if e.obf != nil {
		var err error
		if name, data, err = e.obf.consumer(c.Name, c.Data); err != nil {
			return err
		}
	}
	if e.enc == nil {
		return nil
	}
	return e.enc.WriteConsumer(name, c.Ts, data)
}

// twoPass observes the filtered result per subject first and writes only
// after the source sentinel was reached, so state.json carries exact totals.
// The second pass rewinds the handle held since the first; it re-runs no
// filters and must copy exactly what the first pass resolved. A plain dry
// run stops after Resolve; an obfuscated dry run replays the second pass
// without an encoder so the token and body counters match a real run
func (e *editor) twoPass() error {
	state := newSubjectState(e.o)
	var passed uint64

	dec := NewDecoder(e.src.tee)
	for done := false; !done; {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		item, err := dec.Next()
		if err != nil {
			return err
		}
		e.src.prog.entries++

		switch it := item.(type) {
		case *State:
			e.srcState = it.State
		case *Consumer:
			if e.o.renumber {
				e.report.ConsumersDropped++
			} else {
				e.report.ConsumersKept++
			}
		case *Message:
			ok, body, err := e.evaluate(it)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			passed++
			tomb := false
			if e.o.kvCompact && it.HdrSize > 0 {
				hdr, err := body.headers()
				if err != nil {
					return err
				}
				tomb = isTombstone(hdr)
			}
			state.Observe(it.Subject, it.Seq, storedMsgSize(len(it.Subject), it.HdrSize, it.PayloadSize), tomb)
		case End:
			if e.digest, err = e.src.finishDigest(); err != nil {
				return err
			}
			done = true
		}
	}

	res := state.Resolve()
	e.keptSubjectCount = int(res.subjects)
	if e.o.kvCompact {
		e.report.Dropped.KVCompact = passed - res.msgs
	} else {
		e.report.Dropped.LastPerSubject = passed - res.msgs
	}
	e.report.SubjectStateKeys, e.report.SubjectStateBytes = state.Stats()

	if e.enc == nil && e.obf == nil {
		e.report.Kept, e.bytes, e.firstOut = res.msgs, res.bytes, res.firstSeq
		return nil
	}

	if e.o.betweenPasses != nil {
		e.o.betweenPasses()
	}
	r, err := e.src.rewind()
	if err != nil {
		return err
	}

	dec = NewDecoder(r)
	for {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		item, err := dec.Next()
		if err != nil {
			return err
		}
		e.src.prog.entries++

		switch it := item.(type) {
		case *State:
			if err := e.writeState(it.Ts); err != nil {
				return err
			}
		case *Consumer:
			if e.o.renumber {
				continue
			}
			if err := e.writeConsumer(it); err != nil {
				return err
			}
		case *Message:
			if !state.Keeps(it.Subject, it.Seq) {
				continue
			}
			if err := e.writeMessage(&msgBody{m: it}); err != nil {
				return err
			}
		case End:
			if e.report.Kept != res.msgs || (e.obf == nil && e.bytes != res.bytes) {
				return fmt.Errorf("second pass copied %d messages (%d bytes) but the first pass resolved %d (%d bytes): the source changed during the edit", e.report.Kept, e.bytes, res.msgs, res.bytes)
			}
			if e.enc == nil {
				return nil
			}
			return e.enc.WriteEnd()
		}
	}
}

func (e *editor) filterMessage(m *Message) error {
	ok, body, err := e.evaluate(m)
	if err != nil || !ok {
		return err
	}
	return e.writeMessage(body)
}

// evaluate applies the stateless filters; a rejected message is counted and reported as not ok
func (e *editor) evaluate(m *Message) (bool, *msgBody, error) {
	e.report.SourceMessages++
	e.srcSubjects.add(m.Subject)
	body := &msgBody{m: m}

	kind := e.o.evalMeta(m)
	if kind == keep && e.o.hasHeaderFilters() {
		hdr, err := body.headers()
		if err != nil {
			return false, nil, err
		}
		kind = e.o.evalHeaders(hdr)
	}
	if kind == keep && e.o.hasPayloadFilters() {
		payload, err := body.payload()
		if err != nil {
			return false, nil, err
		}
		kind = e.o.evalPayload(payload)
	}
	if kind != keep {
		return false, body, e.drop(kind, body)
	}

	return true, body, nil
}

func (e *editor) drop(kind dropKind, body *msgBody) error {
	e.report.Dropped.add(kind)
	if !kind.contentFilter() || body.m.HdrSize == 0 {
		return nil
	}
	hdr, err := body.headers()
	if err != nil {
		return err
	}
	if isTombstone(hdr) {
		e.report.TombstonesRemovedByContentFilters++
	}
	return nil
}

func (e *editor) writeMessage(body *msgBody) error {
	m := body.m
	out := *m

	if e.cfg.MaxAge > 0 && time.Unix(0, m.Ts).Add(e.cfg.MaxAge).Before(e.now) {
		e.overAge++
	}

	if e.obf != nil {
		hdr, err := body.headers()
		if err != nil {
			return err
		}
		subject, block, err := e.obf.message(m.Subject, hdr)
		if err != nil {
			return err
		}
		if e.cfg.AllowMsgCounter {
			payload, err := body.payload()
			if err != nil {
				return err
			}
			out.Subject, out.HdrSize, out.Body = subject, int64(len(block)), io.MultiReader(bytes.NewReader(block), bytes.NewReader(payload))
		} else {
			if m.PayloadSize > 0 {
				e.obf.bodies++
			}
			out.Subject, out.HdrSize, out.Body = subject, int64(len(block)), io.MultiReader(bytes.NewReader(block), io.LimitReader(zeroPadding{}, m.PayloadSize))
		}
	} else {
		out.Body = body.reader()
	}

	e.report.Kept++
	if e.keptSubjects != nil {
		e.keptSubjects.add(m.Subject)
	}
	if e.o.renumber {
		out.Seq = e.report.Kept
	}
	if e.report.Kept == 1 {
		e.firstOut = out.Seq
	}
	e.bytes += storedMsgSize(len(out.Subject), out.HdrSize, out.PayloadSize)
	if e.enc == nil {
		return nil
	}

	return e.enc.WriteMessage(&out)
}

// sourceInfo copies a capture's source block into the edited backup, with
// the stream and subjects mapped under obfuscation
func (e *editor) sourceInfo() (*SourceInfo, error) {
	src := e.src.metaFile.Source
	if src == nil || e.obf == nil {
		return src, nil
	}

	out := *src
	var err error
	if out.Stream != "" {
		if out.Stream, err = e.obf.streamName(src.Stream); err != nil {
			return nil, err
		}
	}
	if out.Subjects, err = e.obf.subjects(src.Subjects); err != nil {
		return nil, err
	}
	return &out, nil
}

// srcLastSeq is the source's last sequence. A capture or a single-pass
// renumbered edit has a placeholder last_seq in its archive and the real
// value only in its meta file
func (e *editor) srcLastSeq() uint64 {
	return max(e.srcState.LastSeq, e.src.metaFile.State.LastSeq)
}

// resultRange is the sequence range the restored stream will report: the
// first kept sequence through the source's last (preserve) or K (renumber);
// an edit that kept nothing restores as an empty stream at last+1/last
func (e *editor) resultRange() (first uint64, last uint64) {
	switch {
	case e.o.renumber:
		return 1, e.report.Kept
	case e.report.Kept > 0:
		return e.firstOut, e.srcLastSeq()
	case e.report.SourceMessages == 0:
		return e.srcState.FirstSeq, e.srcState.LastSeq
	default:
		return e.srcLastSeq() + 1, e.srcLastSeq()
	}
}

func (e *editor) result() (*Result, error) {
	e.report.SourceSubjects = e.srcSubjects.count()
	e.report.KeptSubjects = e.keptSubjectCount
	if e.keptSubjects != nil {
		e.report.KeptSubjects = e.keptSubjects.count()
	}

	cfg := e.cfg
	if e.o.renumber {
		cfg.FirstSeq = 0
	}
	if e.obf != nil {
		var err error
		if cfg, err = e.obf.config(cfg); err != nil {
			return nil, err
		}
		e.report.Obfuscation = &ObfuscationReport{TokensMapped: len(e.obf.reverse), BodiesPadded: e.obf.bodies}
	}

	st := api.StreamState{
		Msgs:      e.report.Kept,
		Bytes:     e.bytes,
		Consumers: e.report.ConsumersKept,
	}
	st.FirstSeq, st.LastSeq = e.resultRange()

	if cfg.MaxMsgs > 0 && e.report.Kept > uint64(cfg.MaxMsgs) {
		e.warn("%d messages kept but max_msgs is %d: restore will discard the oldest %d", e.report.Kept, cfg.MaxMsgs, e.report.Kept-uint64(cfg.MaxMsgs))
	}
	if cfg.MaxBytes > 0 && e.bytes > uint64(cfg.MaxBytes) {
		e.warn("%d bytes kept but max_bytes is %d: restore will discard the oldest messages", e.bytes, cfg.MaxBytes)
	}
	if cfg.MaxAge > 0 && e.overAge > 0 {
		e.warn("%d kept messages are already older than max_age %s: restore will discard them", e.overAge, cfg.MaxAge)
	}
	if e.o.lastPerSubject > 0 && cfg.MaxMsgsPer > 0 && int64(e.o.lastPerSubject) > cfg.MaxMsgsPer {
		e.warn("last-per-subject %d exceeds max_msgs_per_subject %d: restore will keep only %d per subject", e.o.lastPerSubject, cfg.MaxMsgsPer, cfg.MaxMsgsPer)
	}

	return &Result{Config: cfg, State: st, Report: e.report}, nil
}

func (e *editor) warn(format string, args ...any) {
	e.report.Warnings = append(e.report.Warnings, fmt.Sprintf(format, args...))
}
