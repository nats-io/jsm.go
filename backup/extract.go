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
	"context"
	"errors"
	"time"

	"github.com/nats-io/jsm.go/api"
)

// Extract reads the backup in srcDir and calls fn with every message the
// filter options keep, in stream order, with the header block exactly as
// stored. An error returned by fn stops the extract and is returned
// unchanged. Renumber, Obfuscate and DryRun are rejected since they describe
// an edited backup
func Extract(ctx context.Context, srcDir string, fn func(msg *api.StoredMsg) error, opts ...EditOption) error {
	if fn == nil {
		return errors.New("Extract requires a message function")
	}
	o := &editOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if err := o.validate(); err != nil {
		return err
	}
	if o.renumber || o.obfuscate || o.dryRun {
		return errors.New("Renumber, Obfuscate and DryRun are not supported by Extract")
	}

	passes := 1
	if o.perSubject() {
		passes = 2
	}
	ed, err := newEditor(ctx, srcDir, o, passes)
	if err != nil {
		return err
	}
	defer ed.src.close()
	ed.sink = fn

	if err := ed.run(nil); err != nil {
		return err
	}
	ed.src.prog.finish()

	return nil
}

func (e *editor) extract(body *msgBody) error {
	buf, err := body.prefix(body.m.HdrSize + body.m.PayloadSize)
	if err != nil {
		return err
	}
	headers, data := buf[:body.m.HdrSize:body.m.HdrSize], buf[body.m.HdrSize:]

	msg := &api.StoredMsg{
		Subject:  body.m.Subject,
		Sequence: body.m.Seq,
		Data:     data,
		Time:     time.Unix(0, body.m.Ts).UTC(),
	}
	if len(headers) > 0 {
		msg.Header = headers
	}

	return e.sink(msg)
}
