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

package api

import (
	"log"
)

type Logger interface {
	Tracef(format string, a ...any)
	Debugf(format string, a ...any)
	Infof(format string, a ...any)
	Warnf(format string, a ...any)
	Errorf(format string, a ...any)
}

type Level uint

const (
	TraceLevel Level = 4
	DebugLevel Level = 3
	InfoLevel  Level = 2
	WarnLevel  Level = 1
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

func (d *dfltLogger) Tracef(format string, a ...any) {
	if d.lvl >= TraceLevel {
		d.logFunc(format, a...)
	}
}

func (d *dfltLogger) Debugf(format string, a ...any) {
	if d.lvl >= DebugLevel {
		d.logFunc(format, a...)
	}
}

func (d *dfltLogger) Infof(format string, a ...any) {
	if d.lvl >= InfoLevel {
		d.logFunc(format, a...)
	}
}

func (d *dfltLogger) Warnf(format string, a ...any) {
	if d.lvl >= WarnLevel {
		d.logFunc(format, a...)
	}
}

func (d *dfltLogger) Errorf(format string, a ...any) {
	if d.lvl >= ErrorLevel {
		d.logFunc(format, a...)
	}
}
