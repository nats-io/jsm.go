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

// Package backup reads, verifies, inspects and edits JetStream stream backups
// on disk without a server connection.
//
// Only the NATS Server 2.15 backup format is supported: an s2 compressed
// NATSARC1 archive named stream.tar.s2 next to a backup.json meta file, the
// layout written by jsm.Stream.SnapshotToDirectory and read by
// jsm.Manager.RestoreSnapshotFromDirectory. Older tar based backups are
// refused.
package backup

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/nats-io/jsm.go/api"
)

const (
	// DataFile is the archive file name inside a backup directory
	DataFile = "stream.tar.s2"
	// MetaFile is the meta file name inside a backup directory
	MetaFile = "backup.json"

	stateEntry     = "state.json"
	consumerPrefix = "consumers/"
)

// ErrNotV2Backup is returned when a source is not a NATS Server 2.15 stream backup
var ErrNotV2Backup = errors.New("not a NATS Server 2.15 stream backup")

// Item is one decoded archive entry: *State, *Consumer, *Message or End
type Item interface {
	item()
}

// State is the state.json entry that leads every backup
type State struct {
	Ts    int64
	State api.StreamState
}

// Consumer is one consumers/<name> entry holding the JSON encoded config and state
type Consumer struct {
	Name string
	Ts   int64
	Data []byte
}

// Message is one stored message; Body yields the headers followed by the payload
// and is only valid until the next call to Decoder.Next
type Message struct {
	Subject     string
	Seq         uint64
	Ts          int64
	HdrSize     int64
	PayloadSize int64
	Body        io.Reader
}

// End is the end-of-backup sentinel
type End struct{}

func (*State) item()    {}
func (*Consumer) item() {}
func (*Message) item()  {}
func (End) item()       {}

// EntryRef identifies an archive entry by position and identity
type EntryRef struct {
	Ordinal int
	Kind    string
	Name    string
	Seq     uint64
}

func (r EntryRef) String() string {
	switch r.Kind {
	case "message":
		return "entry #" + strconv.Itoa(r.Ordinal) + " message " + r.Name + " seq " + strconv.FormatUint(r.Seq, 10)
	case "":
		return "no entries"
	default:
		return "entry #" + strconv.Itoa(r.Ordinal) + " " + r.Kind + " " + r.Name
	}
}

// storedMsgSize mirrors the server's unexported fileStoreMsgSizeRaw
func storedMsgSize(slen int, hlen, mlen int64) uint64 {
	if hlen == 0 {
		return uint64(22 + int64(slen) + mlen + 8)
	}
	return uint64(22 + int64(slen) + 4 + hlen + mlen + 8)
}

func backupPaths(dir string) (data string, meta string, err error) {
	data = filepath.Join(dir, DataFile)
	meta = filepath.Join(dir, MetaFile)
	for _, p := range []string{data, meta} {
		if _, err := os.Stat(p); err != nil {
			return "", "", err
		}
	}
	return data, meta, nil
}
