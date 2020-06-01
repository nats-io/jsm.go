// Copyright 2020 The NATS Authors
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

package jsm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

type snapshotOptions struct {
	file  string
	scb   func(SnapshotProgress)
	rcb   func(RestoreProgress)
	debug bool
	conn  *reqoptions
}

type SnapshotOption func(o *snapshotOptions)

func SnapshotNotify(cb func(SnapshotProgress)) SnapshotOption {
	return func(o *snapshotOptions) {
		o.scb = cb
	}
}

func RestoreNotify(cb func(RestoreProgress)) SnapshotOption {
	return func(o *snapshotOptions) {
		o.rcb = cb
	}
}

// SnapshotConnection sets the connection properties to use during snapshot
func SnapshotConnection(opts ...RequestOption) SnapshotOption {
	return func(o *snapshotOptions) {
		for _, opt := range opts {
			opt(o.conn)
		}
	}
}

// SnapshotDebug enables logging using the standard go logging library
func SnapshotDebug() SnapshotOption {
	return func(o *snapshotOptions) {
		o.debug = true
	}
}

type SnapshotProgress interface {
	// StartTime is when the process started
	StartTime() time.Time
	// EndTime is when the process ended - zero when not completed
	EndTime() time.Time
	// ChunkSize is the size of the data packets sent over NATS
	ChunkSize() int
	// ChunksReceived is how many chunks of ChunkSize were received
	ChunksReceived() uint32
	// BytesReceived is how many Bytes have been received
	BytesReceived() uint64
	// BlocksReceived is how many data storage Blocks have been received
	BlocksReceived() int
	// BlockSize is the size in bytes of each data storage Block
	BlockSize() int
	// BlocksExpected is the number of blocks expected to be received
	BlocksExpected() int
	// BlockBytesReceived is the size of uncompressed block data received
	BlockBytesReceived() uint64
	// HasMetadata indicates if the metadata blocks have been received yet
	HasMetadata() bool
	// HasData indicates if all the data blocks have been received
	HasData() bool
	// BytesPerSecond is the number of bytes received in the last second, 0 during the first second
	BytesPerSecond() uint64
}

type RestoreProgress interface {
	// StartTime is when the process started
	StartTime() time.Time
	// EndTime is when the process ended - zero when not completed
	EndTime() time.Time
	// ChunkSize is the size of the data packets sent over NATS
	ChunkSize() int
	// ChunksSent is the number of chunks of size ChunkSize that was sent
	ChunksSent() uint32
	// ChunksToSend number of chunks of ChunkSize expected to be sent
	ChunksToSend() int
	// BytesSent is the number of bytes sent so far
	BytesSent() uint64
	// BytesPerSecond is the number of bytes received in the last second, 0 during the first second
	BytesPerSecond() uint64
}

type snapshotProgress struct {
	startTime          time.Time
	endTime            time.Time
	chunkSize          int
	chunksReceived     uint32
	chunksSent         uint32
	chunksToSend       int
	bytesReceived      uint64
	bytesSent          uint64
	blocksReceived     int32
	blockSize          int
	blocksExpected     int
	blockBytesReceived uint64
	metadataDone       bool
	dataDone           bool
	bps                uint64 // Bytes per second
	scb                func(SnapshotProgress)
	rcb                func(RestoreProgress)
	sync.Mutex
}

func (sp snapshotProgress) BlockBytesReceived() uint64 {
	return sp.blockBytesReceived
}

func (sp snapshotProgress) ChunksReceived() uint32 {
	return sp.chunksReceived
}

func (sp snapshotProgress) BytesReceived() uint64 {
	return sp.bytesReceived
}

func (sp snapshotProgress) BlocksReceived() int {
	return int(sp.blocksReceived)
}

func (sp snapshotProgress) BlockSize() int {
	return sp.blockSize
}

func (sp snapshotProgress) BlocksExpected() int {
	return sp.blocksExpected
}

func (sp snapshotProgress) HasMetadata() bool {
	return sp.metadataDone
}

func (sp snapshotProgress) HasData() bool {
	return sp.dataDone
}

func (sp snapshotProgress) BytesPerSecond() uint64 {
	if sp.bps == 0 {
		return sp.bytesReceived
	}

	return sp.bps
}

func (sp snapshotProgress) StartTime() time.Time {
	return sp.startTime
}

func (sp snapshotProgress) EndTime() time.Time {
	return sp.endTime
}

func (sp snapshotProgress) ChunkSize() int {
	return sp.chunkSize
}

func (sp snapshotProgress) ChunksToSend() int {
	return sp.chunksToSend
}

func (sp snapshotProgress) ChunksSent() uint32 {
	return sp.chunksSent
}

func (sp snapshotProgress) BytesSent() uint64 {
	return sp.bytesSent
}

func (sp *snapshotProgress) incBlockBytesReceived(c uint64) {
	atomic.AddUint64(&sp.blockBytesReceived, c)
}

func (sp *snapshotProgress) incBytesSent(c uint64) {
	atomic.AddUint64(&sp.bytesSent, c)
}

func (sp *snapshotProgress) incChunksSent(c uint32) {
	atomic.AddUint32(&sp.chunksSent, c)
}

func (sp *snapshotProgress) incBytesReceived(c uint64) {
	atomic.AddUint64(&sp.bytesReceived, c)
}

func (sp *snapshotProgress) incChunksReceived(c uint32) {
	atomic.AddUint32(&sp.chunksReceived, c)
}

func (sp *snapshotProgress) notify() {
	if sp.scb != nil {
		sp.scb(*sp)
	}
	if sp.rcb != nil {
		sp.rcb(*sp)
	}
}

// the tracker will gunzip and untar the stream as it passes by looking
// for the file names in the tar data and based on these will notify the
// caller about blocks received etc
func (sp *snapshotProgress) trackBlockProgress(r io.Reader, debug bool, errc chan error) {
	seenMetaSum := false
	seenMetaInf := false

	zr, err := gzip.NewReader(r)
	if err != nil {
		errc <- fmt.Errorf("progress tracker failed to start gzip: %s", err)
		return
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			errc <- fmt.Errorf("progress tracker received EOF")
			return
		}
		if err != nil {
			errc <- fmt.Errorf("progress tracker received an unexpected error: %s", err)
			return
		}

		if debug {
			log.Printf("Received file %s", hdr.Name)
		}

		if !sp.metadataDone {
			if hdr.Name == "meta.sum" {
				seenMetaSum = true
			}
			if hdr.Name == "meta.inf" {
				seenMetaInf = true
			}
			if seenMetaInf && seenMetaSum {
				sp.metadataDone = true

				// tell the caller soon as we are done with metadata
				sp.notify()
			}
		}

		if strings.HasSuffix(hdr.Name, "blk") {
			br := atomic.AddInt32(&sp.blocksReceived, 1)
			sp.incBlockBytesReceived(uint64(hdr.Size))

			// notify before setting done so callers can easily print the
			// last progress for blocks
			sp.notify()

			if int(br) == sp.blocksExpected {
				sp.dataDone = true
			}
		}
	}
}

func (sp *snapshotProgress) trackBps(ctx context.Context) {
	var lastBytesRecvd uint64 = 0

	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ticker.C:
			received := atomic.LoadUint64(&sp.bytesReceived)
			bps := received - lastBytesRecvd
			lastBytesRecvd = received
			sp.bps = bps
			sp.notify()

		case <-ctx.Done():
			return
		}
	}
}

func RestoreSnapshotFromFile(ctx context.Context, stream string, file string, opts ...SnapshotOption) (RestoreProgress, error) {
	sopts := &snapshotOptions{
		file: file,
		conn: dfltreqoptions(),
	}

	for _, opt := range opts {
		opt(sopts)
	}

	fstat, err := os.Stat(file)
	if err != nil {
		return nil, err
	}

	inf, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer inf.Close()

	var resp api.JSApiStreamRestoreResponse
	err = jsonRequest(fmt.Sprintf(api.JSApiStreamRestoreT, stream), map[string]string{}, &resp, sopts.conn)
	if err != nil {
		return nil, err
	}

	chunkSize := 512 * 1024
	progress := snapshotProgress{
		startTime:    time.Now(),
		chunkSize:    chunkSize,
		chunksToSend: int(fstat.Size()) / chunkSize,
		rcb:          sopts.rcb,
		scb:          sopts.scb,
	}
	defer func() { progress.endTime = time.Now() }()
	go progress.trackBps(ctx)

	nc := sopts.conn.nc
	var chunk [512 * 1024]byte
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		n, err := inf.Read(chunk[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		err = nc.Publish(resp.DeliverSubject, chunk[:n])
		if err != nil {
			return nil, err
		}

		progress.incChunksSent(1)
		progress.incBytesSent(uint64(n))
		progress.notify()
	}

	err = nc.Publish(resp.DeliverSubject, nil)
	if err != nil {
		return nil, err
	}

	err = nc.Flush()
	if err != nil {
		return nil, err
	}

	return &progress, nil
}

// SnapshotToFile creates a backup into gzipped tar file
func (s *Stream) SnapshotToFile(ctx context.Context, file string, consumers bool, opts ...SnapshotOption) (SnapshotProgress, error) {
	sopts := &snapshotOptions{
		file: file,
		conn: s.cfg.conn,
	}

	for _, opt := range opts {
		opt(sopts)
	}

	if sopts.debug {
		log.Printf("Starting backup of %q to %q", s.Name(), file)
	}

	of, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	defer of.Close()

	ib := nats.NewInbox()
	req := api.JSApiStreamSnapshotRequest{
		DeliverSubject: ib,
		NoConsumers:    !consumers,
		ChunkSize:      512 * 1024,
	}

	var resp api.JSApiStreamSnapshotResponse
	err = jsonRequest(fmt.Sprintf(api.JSApiStreamSnapshotT, s.Name()), req, &resp, sopts.conn)
	if err != nil {
		return nil, err
	}

	errc := make(chan error)
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()

	progress := snapshotProgress{
		startTime:      time.Now(),
		chunkSize:      req.ChunkSize,
		blockSize:      resp.BlkSize,
		blocksExpected: resp.NumBlks,
		scb:            sopts.scb,
		rcb:            sopts.rcb,
	}
	defer func() { progress.endTime = time.Now() }()
	go progress.trackBps(sctx)

	// set up a multi writer that writes to file and the progress monitor
	// if required else we write directly to the file and be done with it
	trackingR, trackingW := net.Pipe()
	defer trackingR.Close()
	defer trackingW.Close()
	go progress.trackBlockProgress(trackingR, sopts.debug, errc)

	writer := io.MultiWriter(of, trackingW)

	// tell the caller we are starting and what to expect
	progress.notify()

	sub, err := sopts.conn.nc.Subscribe(ib, func(m *nats.Msg) {
		if len(m.Data) == 0 {
			m.Sub.Unsubscribe()
			cancel()
			return
		}

		progress.incBytesReceived(uint64(len(m.Data)))
		progress.incChunksReceived(1)

		n, err := writer.Write(m.Data)
		if err != nil {
			errc <- err
			return
		}
		if n != len(m.Data) {
			errc <- fmt.Errorf("failed to write %d bytes to %s, only wrote %d", len(m.Data), file, n)
			return
		}
	})
	if err != nil {
		return &progress, err
	}
	defer sub.Unsubscribe()

	select {
	case err := <-errc:
		if sopts.debug {
			log.Printf("Snapshot Error: %s", err)
		}

		return &progress, err
	case <-sctx.Done():
		return &progress, nil
	}
}
