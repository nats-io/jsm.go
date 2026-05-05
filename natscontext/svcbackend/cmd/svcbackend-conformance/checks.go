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

package main

// AllChecks returns the full PROTOCOL.md v1 conformance suite in the
// order they should run. Later checks may rely on earlier checks having
// demonstrated that the basic wiring works (e.g. atomicity checks
// assume crypto is sound).
func AllChecks() []Check {
	var out []Check
	out = append(out, subjectChecks()...)
	out = append(out, cryptoChecks()...)
	out = append(out, nameChecks()...)
	out = append(out, envelopeChecks()...)
	out = append(out, behaviorChecks()...)
	out = append(out, atomicityChecks()...)
	out = append(out, logChecks()...)
	return out
}
