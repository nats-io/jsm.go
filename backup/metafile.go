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
	"encoding/json"
	"fmt"
	"os"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

// EditInfo is the "edit" block an edited backup carries in its meta file.
// An edited backup is a derived artifact and this block is its provenance:
// SourceDigest is sha256 of the source stream.tar.s2, hashed while pass 1
// streams it, so with the recorded options and tool version anyone holding
// the source can re-run the edit, expect byte-identical output, and settle
// which archive a shared backup was derived from. The digest is deliberate
// under --obfuscate: it confirms to someone already holding the original
// that the scrubbed copy came from it, and reveals nothing to anyone else
type EditInfo struct {
	Version      string   `json:"version,omitempty"`
	Options      []string `json:"options"`
	SourceDigest string   `json:"source_digest"`
	Obfuscated   bool     `json:"obfuscated,omitempty"`
}

type metaFile struct {
	Config api.StreamConfig `json:"config"`
	State  api.StreamState  `json:"state"`
	Edit   *EditInfo        `json:"edit,omitempty"`
}

func readMetaFile(path string) (*metaFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var mf metaFile
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if mf.Config.Name == "" {
		return nil, fmt.Errorf("%s: config has no stream name", path)
	}
	if !jsm.IsValidName(mf.Config.Name) {
		return nil, fmt.Errorf("%s: invalid stream name %q", path, mf.Config.Name)
	}
	if mf.Config.Storage == api.MemoryStorage {
		return nil, fmt.Errorf("%s: %w", path, jsm.ErrMemoryStreamNotSupported)
	}

	return &mf, nil
}

func (mf *metaFile) marshal() ([]byte, error) {
	return json.MarshalIndent(mf, "", "  ")
}
