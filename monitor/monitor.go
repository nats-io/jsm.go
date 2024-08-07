// Copyright 2023 The NATS Authors
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

package monitor

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

type Status string

var (
	OKStatus       Status = "OK"
	WarningStatus  Status = "WARNING"
	CriticalStatus Status = "CRITICAL"
	UnknownStatus  Status = "UNKNOWN"
)

func newTableWriter(title string) table.Writer {
	tbl := table.NewWriter()
	tbl.SetStyle(table.StyleRounded)
	tbl.Style().Title.Align = text.AlignCenter
	tbl.Style().Format.Header = text.FormatDefault

	if title != "" {
		tbl.SetTitle(title)
	}

	return tbl
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
var passwordRunes = append(letterRunes, []rune("@#_-%^&()")...)

func randomPassword(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = passwordRunes[rand.Intn(len(passwordRunes))]
	}

	return string(b)
}

func fileAccessible(f string) (bool, error) {
	stat, err := os.Stat(f)
	if err != nil {
		return false, err
	}

	if stat.IsDir() {
		return false, fmt.Errorf("is a directory")
	}

	file, err := os.Open(f)
	if err != nil {
		return false, err
	}
	file.Close()

	return true, nil
}

func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}
