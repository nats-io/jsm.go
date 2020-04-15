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

// +build ignore

package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"text/template"
	"time"
)

var schemasFileTemplate = `// auto generated {{.Now}}

package api

import (
	"encoding/base64"

)

var schemas map[string][]byte

func init() {
	schemas = make(map[string][]byte)

{{- range . }}
	schemas["{{ .T }}"], _ = base64.StdEncoding.DecodeString("{{ .S }}")
{{- end }}
}
`

type schema struct {
	T string
	S string
	U string
}

type schemas []*schema

func (s schemas) Now() string {
	return fmt.Sprintf("%s", time.Now())
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func goFmt(file string) error {
	c := exec.Command("go", "fmt", file)
	out, err := c.CombinedOutput()
	if err != nil {
		log.Printf("go fmt failed: %s", string(out))
	}

	return err
}

func getSchame(u string) (string, error) {
	log.Printf("Fetching %s", u)
	result, err := http.Get(u)
	if err != nil {
		return "", err
	}

	if result.StatusCode != 200 {
		return "", fmt.Errorf("got HTTP status: %d: %s", result.StatusCode, result.Status)
	}

	defer result.Body.Close()

	data, err := ioutil.ReadAll(result.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}
func main() {
	s := schemas{
		&schema{
			T: "io.nats.jetstream.api.v1.consumer_configuration",
			U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_configuration.json",
		},
		&schema{
			T: "io.nats.jetstream.api.v1.stream_configuration",
			U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_configuration.json",
		},
		&schema{
			T: "io.nats.jetstream.api.v1.stream_template_configuration",
			U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_template_configuration.json",
		},
	}

	for _, i := range s {
		body, err := getSchame(i.U)
		panicIfErr(err)
		i.S = body
	}

	t, err := template.New("schemas").Parse(schemasFileTemplate)
	panicIfErr(err)

	out, err := os.Create("api/schemas.go")
	panicIfErr(err)

	err = t.Execute(out, s)
	panicIfErr(err)

	out.Close()
	err = goFmt(out.Name())
	panicIfErr(err)
}
