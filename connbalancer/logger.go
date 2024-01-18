// Copyright 2024 The NATS Authors
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

package connbalancer

import (
	"log"
)

type Logger interface {
	Trace(format string, a ...any)
	Debug(format string, a ...any)
	Info(format string, a ...any)
	Error(format string, a ...any)
}

type Level uint

const (
	TraceLevel Level = 3
	DebugLevel Level = 2
	InfoLevel  Level = 1
	ErrorLevel Level = 0
)

type dfltLogger struct {
	logFunc func(format string, a ...any)
	lvl     Level
}

func NewDefaultLogger(level Level) Logger {
	return &dfltLogger{lvl: level, logFunc: log.Printf}
}

func NewDiscardLogger() Logger {
	return &dfltLogger{lvl: ErrorLevel, logFunc: func(format string, a ...any) {}}
}

func (d *dfltLogger) Trace(format string, a ...any) {
	if d.lvl >= TraceLevel {
		d.logFunc(format, a...)
	}
}

func (d *dfltLogger) Debug(format string, a ...any) {
	if d.lvl >= DebugLevel {
		d.logFunc(format, a...)
	}
}

func (d *dfltLogger) Info(format string, a ...any) {
	if d.lvl >= InfoLevel {
		d.logFunc(format, a...)
	}
}

func (d *dfltLogger) Error(format string, a ...any) {
	if d.lvl >= ErrorLevel {
		d.logFunc(format, a...)
	}
}
