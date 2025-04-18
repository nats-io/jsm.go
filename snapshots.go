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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/klauspost/compress/s2"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

type snapshotOptions struct {
	dir           string
	metaFile      string
	dataFile      string
	dataFileSize  int64
	scb           func(SnapshotProgress)
	rcb           func(RestoreProgress)
	debug         bool
	consumers     bool
	jsck          bool
	chunkSz       int
	progress      bool
	restoreConfig *api.StreamConfig
}

const (
	StatusHdr      string = "Status"
	DescriptionHdr string = "Description"
)

// ErrMemoryStreamNotSupported is an error indicating a memory stream was being snapshotted which is not supported
var ErrMemoryStreamNotSupported error = errors.New("memory streams do not support snapshots")

type SnapshotOption func(o *snapshotOptions)

// SnapshotConsumers includes consumer configuration and state in backups
func SnapshotConsumers() SnapshotOption {
	return func(o *snapshotOptions) {
		o.consumers = true
	}
}

// SnapshotHealthCheck performs a health check prior to starting the snapshot
func SnapshotHealthCheck() SnapshotOption {
	return func(o *snapshotOptions) {
		o.jsck = true
	}
}

// SnapshotNotify notifies cb about progress of the snapshot operation
func SnapshotNotify(cb func(SnapshotProgress)) SnapshotOption {
	return func(o *snapshotOptions) {
		o.scb = cb
	}
}

// RestoreNotify notifies cb about progress of the restore operation
func RestoreNotify(cb func(RestoreProgress)) SnapshotOption {
	return func(o *snapshotOptions) {
		o.rcb = cb
	}
}

// SnapshotDebug enables logging using the standard go logging library
func SnapshotDebug() SnapshotOption {
	return func(o *snapshotOptions) {
		o.debug = true
	}
}

// RestoreConfiguration overrides the configuration used to restore
func RestoreConfiguration(cfg api.StreamConfig) SnapshotOption {
	return func(o *snapshotOptions) {
		o.restoreConfig = &cfg
	}
}

// SnapshotChunkSize sets the size of messages holding data the server will send, good values are 64KB and 128KB
func SnapshotChunkSize(sz int) SnapshotOption {
	return func(o *snapshotOptions) {
		o.chunkSz = sz
	}
}

// WithProgress enables progress tracking
func WithProgress(sz int) SnapshotOption {
	return func(o *snapshotOptions) {
		o.progress = true
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
	// BytesExpected is how many Bytes we should be receiving
	BytesExpected() uint64
	// BytesReceived is how many Bytes have been received
	BytesReceived() uint64
	// UncompressedBytesReceived is the number of bytes received uncompressed
	UncompressedBytesReceived() uint64
	// BytesPerSecond is the number of bytes received in the last second, 0 during the first second
	BytesPerSecond() uint64
	// HealthCheck indicates if health checking was requested
	HealthCheck() bool
	// Finished will be true after all data have been written
	Finished() bool
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
	startTime                 time.Time
	endTime                   time.Time
	healthCheck               bool
	chunkSize                 int
	chunksReceived            uint32
	chunksSent                uint32
	chunksToSend              int
	dataSize                  int64
	bytesReceived             uint64
	uncompressedBytesReceived uint64
	bytesExpected             uint64
	bytesSent                 uint64
	finished                  bool
	sending                   bool   // if we are sending data, this is a hint for bps calc
	bps                       uint64 // Bytes per second
	scb                       func(SnapshotProgress)
	rcb                       func(RestoreProgress)

	sync.Mutex
}

func (sp *snapshotProgress) Finished() bool {
	sp.Lock()
	defer sp.Unlock()

	return sp.finished
}

func (sp *snapshotProgress) HealthCheck() bool {
	sp.Lock()
	defer sp.Unlock()

	return sp.healthCheck
}

func (sp *snapshotProgress) ChunksReceived() uint32 {
	sp.Lock()
	defer sp.Unlock()

	return sp.chunksReceived
}

func (sp *snapshotProgress) BytesExpected() uint64 {
	sp.Lock()
	defer sp.Unlock()

	return sp.bytesExpected
}

func (sp *snapshotProgress) BytesReceived() uint64 {
	sp.Lock()
	defer sp.Unlock()

	return sp.bytesReceived
}

func (sp *snapshotProgress) UncompressedBytesReceived() uint64 {
	sp.Lock()
	defer sp.Unlock()

	return sp.uncompressedBytesReceived
}

func (sp *snapshotProgress) BytesPerSecond() uint64 {
	sp.Lock()
	defer sp.Unlock()

	if sp.bps > 0 {
		return sp.bps
	}

	if sp.sending {
		return sp.bytesSent
	}

	return sp.bytesReceived
}

func (sp *snapshotProgress) StartTime() time.Time {
	sp.Lock()
	defer sp.Unlock()

	return sp.startTime
}

func (sp *snapshotProgress) EndTime() time.Time {
	sp.Lock()
	defer sp.Unlock()

	return sp.endTime
}

func (sp *snapshotProgress) ChunkSize() int {
	sp.Lock()
	defer sp.Unlock()

	return sp.chunkSize
}

func (sp *snapshotProgress) ChunksToSend() int {
	sp.Lock()
	defer sp.Unlock()

	return sp.chunksToSend
}

func (sp *snapshotProgress) ChunksSent() uint32 {
	sp.Lock()
	defer sp.Unlock()

	return sp.chunksSent
}

func (sp *snapshotProgress) BytesSent() uint64 {
	sp.Lock()
	defer sp.Unlock()

	return sp.bytesSent
}

func (sp *snapshotProgress) notify() {
	if sp.scb != nil {
		sp.scb(sp)
	}
	if sp.rcb != nil {
		sp.rcb(sp)
	}
}

// the tracker will uncompress and untar the stream keeping count of bytes received etc
func (sp *snapshotProgress) trackBlockProgress(r io.Reader, debug bool, errc chan error) {
	sr := s2.NewReader(r)

	for {
		b := make([]byte, sp.chunkSize)
		i, err := sr.Read(b)
		if err != nil {
			sp.notify()
			return
		}

		sp.Lock()
		sp.uncompressedBytesReceived += uint64(i)
		sp.Unlock()

		sp.notify()
	}
}

func (sp *snapshotProgress) trackBps(ctx context.Context) {
	var lastBytes uint64 = 0

	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ticker.C:
			sp.Lock()
			if sp.sending {
				sent := sp.bytesSent
				sp.bps = sent - lastBytes
				lastBytes = sent
			} else {
				received := sp.bytesReceived
				sp.bps = received - lastBytes
				lastBytes = received
			}
			sp.Unlock()

			sp.notify()

		case <-ctx.Done():
			return
		}
	}
}

func (s *Stream) createSnapshot(ctx context.Context, dataBuffer, metadataBuffer io.WriteCloser, sopts *snapshotOptions) (SnapshotProgress, error) {
	defer dataBuffer.Close()
	defer metadataBuffer.Close()

	if s.Storage() == api.MemoryStorage {
		return nil, ErrMemoryStreamNotSupported
	}

	if sopts.debug {
		log.Printf("Starting backup of %q", s.Name())
	}

	ib := s.mgr.nc.NewRespInbox()
	req := api.JSApiStreamSnapshotRequest{
		DeliverSubject: ib,
		NoConsumers:    !sopts.consumers,
		CheckMsgs:      sopts.jsck,
		ChunkSize:      sopts.chunkSz,
	}

	var resp api.JSApiStreamSnapshotResponse
	err := s.mgr.jsonRequest(fmt.Sprintf(api.JSApiStreamSnapshotT, s.Name()), req, &resp)
	if err != nil {
		return nil, err
	}

	errc := make(chan error)
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var writer io.Writer

	var progress *snapshotProgress
	if sopts.progress {
		progress = &snapshotProgress{
			startTime:     time.Now(),
			chunkSize:     req.ChunkSize,
			dataSize:      sopts.dataFileSize,
			bytesExpected: resp.State.Bytes,
			scb:           sopts.scb,
			rcb:           sopts.rcb,
			healthCheck:   sopts.jsck,
		}
		defer func() { progress.endTime = time.Now() }()
		go progress.trackBps(sctx)

		// set up a multi writer that writes to file and the progress monitor
		// if required else we write directly to the file and be done with it
		trackingR, trackingW := net.Pipe()
		defer trackingR.Close()
		defer trackingW.Close()
		go progress.trackBlockProgress(trackingR, sopts.debug, errc)

		writer = io.MultiWriter(dataBuffer, trackingW)

		// tell the caller we are starting and what to expect
		progress.notify()
	} else {
		writer = io.MultiWriter(dataBuffer)
	}

	sub, err := s.mgr.nc.Subscribe(ib, func(m *nats.Msg) {
		if len(m.Data) == 0 {
			statusValue := m.Header.Get(StatusHdr)

			// if the server returns a non-204 status code in the message header, return an error
			if statusValue != "" && !strings.Contains(statusValue, "204") {
				descriptionValue := m.Header.Get(DescriptionHdr)
				errc <- fmt.Errorf("%s %s", statusValue, descriptionValue)
			}

			m.Sub.Unsubscribe()
			cancel()
			return
		}

		if sopts.progress {
			progress.Lock()
			progress.bytesReceived += uint64(len(m.Data))
			progress.chunksReceived++
			progress.Unlock()
		}

		n, err := writer.Write(m.Data)
		if err != nil {
			errc <- err
			return
		}
		if n != len(m.Data) {
			errc <- fmt.Errorf("failed to write %d bytes to %s, only wrote %d", len(m.Data), sopts.dataFile, n)
			return
		}

		if m.Reply != "" {
			if sopts.debug && sopts.progress {
				progress.Lock()
				log.Printf("Responding to server subject %s %s chunks, last chunk size %s", m.Reply, humanize.Comma(int64(progress.chunksReceived)), humanize.IBytes(uint64(len(m.Data))))
				progress.Unlock()
			}

			m.Respond(nil)
		}
	})
	if err != nil {
		return progress, err
	}
	defer sub.Unsubscribe()
	sub.SetPendingLimits(-1, -1)

	select {
	case err := <-errc:
		if sopts.debug {
			log.Printf("Snapshot Error: %s", err)
		}

		return progress, err
	case <-sctx.Done():
		meta := map[string]any{
			"config": resp.Config,
			"state":  resp.State,
		}
		mj, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return nil, err
		}

		metadataBuffer.Write(mj)

		if sopts.progress {
			progress.finished = true
			progress.notify()
		}

		return progress, nil
	}
}

// SnapshotToDirectory creates a backup into s2 compressed tar file
func (s *Stream) SnapshotToDirectory(ctx context.Context, dir string, opts ...SnapshotOption) (SnapshotProgress, error) {
	sopts := &snapshotOptions{
		dir:       dir,
		dataFile:  filepath.Join(dir, "stream.tar.s2"),
		metaFile:  filepath.Join(dir, "backup.json"),
		jsck:      false,
		consumers: false,
		chunkSz:   128 * 1024,
		progress:  true,
	}

	for _, opt := range opts {
		opt(sopts)
	}

	err := os.MkdirAll(sopts.dir, 0700)
	if err != nil {
		return nil, err
	}

	mf, err := os.Create(sopts.metaFile)
	if err != nil {
		return nil, err
	}

	df, err := os.Create(sopts.dataFile)
	if err != nil {
		return nil, err
	}

	return s.createSnapshot(ctx, df, mf, sopts)
}

// SnapshotToBuffer creates a compressed s2 backup and writes to an io.Writer
func (s *Stream) SnapshotToBuffer(ctx context.Context, dataBuffer, metadataBuffer io.WriteCloser, opts ...SnapshotOption) (SnapshotProgress, error) {
	sopts := &snapshotOptions{
		jsck:      false,
		consumers: false,
		chunkSz:   128 * 1024,
		progress:  false,
	}

	for _, opt := range opts {
		opt(sopts)
	}

	return s.createSnapshot(ctx, dataBuffer, metadataBuffer, sopts)
}

func (m *Manager) restoreSnapshot(ctx context.Context, stream string, dataReader, metadataReader io.ReadCloser, sopts *snapshotOptions) (RestoreProgress, *api.StreamState, error) {
	defer dataReader.Close()
	defer metadataReader.Close()

	req := api.JSApiStreamRestoreRequest{}
	mj, err := io.ReadAll(metadataReader)
	if err != nil {
		return nil, nil, err
	}
	err = json.Unmarshal(mj, &req)
	if err != nil {
		return nil, nil, err
	}

	// allow full config override
	if sopts.restoreConfig != nil {
		req.Config = *sopts.restoreConfig
	}

	// allow just stream name override
	//
	// this used to be allowed but turns out the server support for this was
	// not up to scratch and fixing it would mean having to rebuild and re-checksum
	// every message, so for now we error here instead
	if req.Config.Name != stream {
		return nil, nil, fmt.Errorf("stream name may not be changed during restore")
	}

	if req.Config.Storage == api.MemoryStorage {
		return nil, nil, ErrMemoryStreamNotSupported
	}

	var resp api.JSApiStreamRestoreResponse
	err = m.jsonRequest(fmt.Sprintf(api.JSApiStreamRestoreT, req.Config.Name), req, &resp)
	if err != nil {
		return nil, nil, err
	}

	var progress *snapshotProgress
	var notifyInterval uint32

	if sopts.progress {
		progress = &snapshotProgress{
			startTime:    time.Now(),
			chunkSize:    sopts.chunkSz,
			chunksToSend: 1 + int(sopts.dataFileSize)/sopts.chunkSz,
			sending:      true,
			rcb:          sopts.rcb,
			scb:          sopts.scb,
		}
		defer func() { progress.endTime = time.Now() }()
		go progress.trackBps(ctx)

		// in debug notify ~20ish times
		notifyInterval = uint32(1)
		if progress.chunksToSend >= 20 {
			notifyInterval = uint32(math.Ceil(float64(progress.chunksToSend) / 20))
		}

		// send initial notify to inform what to expect
		progress.notify()
	}

	if sopts.debug {
		log.Printf("Starting restore of %q from %s using %d chunks", req.Config.Name, sopts.dataFile, progress.chunksToSend)
	}

	nc := m.nc
	var chunk [64 * 1024]byte
	var cresp *nats.Msg

	for {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		n, err := dataReader.Read(chunk[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		cresp, err = nc.Request(resp.DeliverSubject, chunk[:n], m.timeout)
		if err != nil {
			return nil, nil, err
		}
		if IsErrorResponse(cresp) {
			return nil, nil, fmt.Errorf("restore failed: %q", cresp.Data)
		}

		if sopts.progress {
			if sopts.debug && progress.chunksSent > 0 && progress.chunksSent%notifyInterval == 0 {
				log.Printf("Sent %d chunks", progress.chunksSent)
			}

			progress.Lock()
			progress.chunksSent++
			progress.bytesSent += uint64(n)
			progress.Unlock()

			progress.notify()
		}
	}

	if sopts.debug && sopts.progress {
		log.Printf("Sent %d chunks, server will now restore the snapshot, this might take a long time", progress.chunksSent)
	}

	// very long timeout as the server is doing the restore here and might take any mount of time
	cresp, err = nc.Request(resp.DeliverSubject, nil, time.Hour)
	if err != nil {
		return nil, nil, err
	}

	kind, finalr, err := api.ParseMessage(cresp.Data)
	if err != nil {
		return nil, nil, err
	}

	if kind != "io.nats.jetstream.api.v1.stream_create_response" {
		return nil, nil, fmt.Errorf("invalid final response, expected a io.nats.jetstream.api.v1.stream_create_response message but got %q", kind)
	}

	createResp, ok := finalr.(*api.JSApiStreamCreateResponse)
	if !ok {
		return nil, nil, fmt.Errorf("invalid final response type")
	}
	if createResp.IsError() {
		return nil, nil, createResp.ToError()
	}

	return progress, &createResp.State, nil
}

func (m *Manager) RestoreSnapshotFromDirectory(ctx context.Context, stream string, dir string, opts ...SnapshotOption) (RestoreProgress, *api.StreamState, error) {
	sopts := &snapshotOptions{
		dir:      dir,
		dataFile: filepath.Join(dir, "stream.tar.s2"),
		metaFile: filepath.Join(dir, "backup.json"),
		chunkSz:  64 * 1024,
		progress: true,
	}

	fstat, err := os.Stat(sopts.dataFile)
	if err != nil {
		return nil, nil, err
	}

	sopts.dataFileSize = fstat.Size()

	for _, opt := range opts {
		opt(sopts)
	}

	df, err := os.Open(sopts.dataFile)
	if err != nil {
		return nil, nil, err
	}
	defer df.Close()

	mf, err := os.Open(sopts.metaFile)
	if err != nil {
		return nil, nil, err
	}

	return m.restoreSnapshot(ctx, stream, df, mf, sopts)
}

// RestoreSnapshotFromBuffer restores a stream from a s2 compressed backup read from an io.Reader.
func (m *Manager) RestoreSnapshotFromBuffer(ctx context.Context, stream string, dataReader, metadataReader io.ReadCloser, opts ...SnapshotOption) (*api.StreamState, error) {
	sopts := &snapshotOptions{
		chunkSz:  64 * 1024,
		progress: false,
	}

	for _, opt := range opts {
		opt(sopts)
	}

	_, ss, err := m.restoreSnapshot(ctx, stream, dataReader, metadataReader, sopts)
	return ss, err
}
