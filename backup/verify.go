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
	"fmt"
	"os"
	"time"

	"github.com/nats-io/jsm.go/api"
)

// VerifyReport summarizes a scan of a backup directory
type VerifyReport struct {
	// Entries is the number of archive entries that decoded cleanly
	Entries   int
	Consumers int
	Messages  uint64
	// FirstSeq and LastSeq are the first and last message sequences found, 0 when empty
	FirstSeq uint64
	LastSeq  uint64
	// Complete is true once the end-of-backup sentinel was reached
	Complete bool
	// LastGood identifies the last entry that decoded cleanly
	LastGood EntryRef
}

// InfoReport describes a backup directory from a full scan of its archive
type InfoReport struct {
	// Config is the stream configuration from the meta file
	Config api.StreamConfig `json:"config"`
	// Declared is the state.json inside the archive; its messages and bytes
	// are advisory and may be 0 in an edited backup
	Declared api.StreamState `json:"declared"`
	// DeclaredCountsAdvisory is true when Declared carries 0 messages or bytes
	// while the archive holds messages
	DeclaredCountsAdvisory bool `json:"declared_counts_advisory"`
	// Edit is the meta file's edit block when the backup was produced by Edit
	Edit *EditInfo `json:"edit,omitempty"`
	// Consumers are the consumer names in archive order
	Consumers []string `json:"consumers"`
	// Messages is the number of messages in the archive
	Messages uint64 `json:"messages"`
	// Bytes is the storage the messages will occupy on restore, per the file store's accounting
	Bytes uint64 `json:"bytes"`
	// FirstSeq and LastSeq bound the message sequences in the archive, 0 when empty
	FirstSeq uint64 `json:"first_seq"`
	LastSeq  uint64 `json:"last_seq"`
	// FirstTime and LastTime bound the message timestamps, zero when empty
	FirstTime time.Time `json:"first_time"`
	LastTime  time.Time `json:"last_time"`
}

// Verify checks that dir holds a complete, restorable 2.15 stream backup: a
// parseable meta file and an archive whose entries satisfy every invariant the
// server's restore enforces, terminated by the sentinel. A nil error means
// valid; when the archive fails part way the report and the error name the
// last good entry
func Verify(dir string) (*VerifyReport, error) {
	_, report, err := scan(dir)
	return report, err
}

// Info scans a backup directory to the sentinel and reports what it holds
func Info(dir string) (*InfoReport, error) {
	info, _, err := scan(dir)
	return info, err
}

func scan(dir string) (*InfoReport, *VerifyReport, error) {
	data, meta, err := backupPaths(dir)
	if err != nil {
		return nil, nil, err
	}

	mf, err := readMetaFile(meta)
	if err != nil {
		return nil, nil, err
	}

	f, err := os.Open(data)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	info := &InfoReport{Config: mf.Config, Edit: mf.Edit}
	report := &VerifyReport{}
	dec := NewDecoder(f)
	for {
		item, err := dec.Next()
		if err != nil {
			report.LastGood = dec.LastGood()
			report.FirstSeq = info.FirstSeq
			report.LastSeq = info.LastSeq
			return nil, report, fmt.Errorf("%s: %w (last good entry: %s)", data, err, report.LastGood)
		}

		report.Entries++
		switch it := item.(type) {
		case *State:
			info.Declared = it.State
		case *Consumer:
			info.Consumers = append(info.Consumers, it.Name)
			report.Consumers++
		case *Message:
			info.Messages++
			report.Messages++
			info.Bytes += storedMsgSize(len(it.Subject), it.HdrSize, it.PayloadSize)
			if info.FirstSeq == 0 {
				info.FirstSeq = it.Seq
				info.FirstTime = time.Unix(0, it.Ts).UTC()
			}
			info.LastSeq = it.Seq
			info.LastTime = time.Unix(0, it.Ts).UTC()
		case End:
			report.Complete = true
			report.LastGood = dec.LastGood()
			report.FirstSeq = info.FirstSeq
			report.LastSeq = info.LastSeq
			info.DeclaredCountsAdvisory = info.Messages > 0 && (info.Declared.Msgs == 0 || info.Declared.Bytes == 0)
			return info, report, nil
		}
	}
}
