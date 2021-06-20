// Copyright 2021 The NATS Authors
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

package kv

import (
	"log"
)

// Logger is a custom logger
type Logger interface {
	Debugf(format string, a ...interface{})
	Infof(format string, a ...interface{})
	WarnF(format string, a ...interface{})
	ErrorF(format string, a ...interface{})
}

type stdLogger struct{}

func (s *stdLogger) Debugf(format string, a ...interface{}) {
	log.Printf(format, a...)
}
func (s *stdLogger) Infof(format string, a ...interface{}) {
	log.Printf(format, a...)
}
func (s *stdLogger) WarnF(format string, a ...interface{}) {
	log.Printf(format, a...)
}
func (s *stdLogger) ErrorF(format string, a ...interface{}) {
	log.Printf(format, a...)
}
