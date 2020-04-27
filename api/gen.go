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
	"encoding/json"
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

var schemaTypes = map[string]func() interface{}{
{{- range . }}
{{- if .St }}
    "{{ .T }}": func() interface{} { return &{{ .St }}{} },
{{- end }}
{{- end }}
	"io.nats.unknown_event":                     func() interface{} { return &UnknownEvent{} },
}

func init() {
	schemas = make(map[string][]byte)

{{- range . }}
	schemas["{{ .T }}"], _ = base64.StdEncoding.DecodeString("{{ .S }}")
{{- end }}
}
`

type schema struct {
	T  string // type
	S  string // schema
	U  string // url
	St string // struct
}

type schemas []*schema

type idDetect struct {
	ID    string `json:"$id"`
	Title string `json:"title"`
}

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

func getSchame(u string) (title string, id string, body string, err error) {
	log.Printf("Fetching %s", u)
	result, err := http.Get(u)
	if err != nil {
		return "", "", "", err
	}

	if result.StatusCode != 200 {
		return "", "", "", fmt.Errorf("got HTTP status: %d: %s", result.StatusCode, result.Status)
	}

	defer result.Body.Close()

	data, err := ioutil.ReadAll(result.Body)
	if err != nil {
		return "", "", "", err
	}

	idt := &idDetect{}
	err = json.Unmarshal(data, idt)
	panicIfErr(err)

	log.Printf("Detected %+v", *idt)
	return idt.Title, idt.ID, base64.StdEncoding.EncodeToString(data), nil
}

func main() {
	s := schemas{
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/server/advisory/v1/client_connect.json", St: "ConnectEventMsg"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/server/advisory/v1/client_disconnect.json", St: "DisconnectEventMsg"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/server/metric/v1/service_latency.json", St: "ServiceLatency"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/api_audit.json", St: "JetStreamAPIAudit"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/max_deliver.json", St: "ConsumerDeliveryExceededAdvisory"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/metric/v1/consumer_ack.json", St: "ConsumerAckMetric"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_configuration.json", St: "ConsumerConfig"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_configuration.json", St: "StreamConfig"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_template_configuration.json", St: "StreamTemplateConfig"},
		&schema{
			U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/definitions.json",
			T: "io.nats.jetstream.api.v1.definitions",
		},
	}

	for _, i := range s {
		title, _, body, err := getSchame(i.U)
		panicIfErr(err)

		i.S = body
		if i.T == "" {
			i.T = title
		}
	}

	t, err := template.New("schemas").Parse(schemasFileTemplate)
	panicIfErr(err)

	out, err := os.Create("api/schemas_generated.go")
	panicIfErr(err)

	err = t.Execute(out, s)
	panicIfErr(err)

	out.Close()
	err = goFmt(out.Name())
	panicIfErr(err)
}
