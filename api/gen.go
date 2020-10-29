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
	"strings"
	"text/template"
	"time"

	"github.com/nats-io/jsm.go/api"
)

var schemasFileTemplate = `// auto generated {{.Now}}

package api

import (
	"encoding/base64"
	srvadvisory "github.com/nats-io/jsm.go/api/server/advisory"
	srvmetric "github.com/nats-io/jsm.go/api/server/metric"
	jsadvisory "github.com/nats-io/jsm.go/api/jetstream/advisory"
    jsmetric "github.com/nats-io/jsm.go/api/jetstream/metric"
	jsapi "github.com/nats-io/jsm.go/api/jetstream/api"
)

var schemas map[string][]byte

var schemaTypes = map[string]func() interface{}{
{{- range . }}
{{- if .St }}
    "{{ .T }}": func() interface{} { return &{{ .St }}{} },
{{- end }}
{{- end }}
	"io.nats.unknown_message": func() interface{} { return &UnknownMessage{} },
}

{{- range . }}
{{- if .ShouldAddValidator }}
// Validate performs a JSON Schema validation of the configuration
func (t {{ .St }}) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type {{ .T }}
func (t {{ .St }}) SchemaType() string {
	return "{{ .T }}"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t {{ .St }}) SchemaID() string {
	return "{{ .SchemaURL }}"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t {{ .St }}) Schema() []byte {
	return schemas[t.SchemaType()]
}

{{- end }}
{{- end }}

func init() {
	schemas = make(map[string][]byte)

{{- range . }}
	schemas["{{ .T }}"], _ = base64.StdEncoding.DecodeString("{{ .S }}")
{{- end }}
}
`

type validator interface {
	Validate() (valid bool, errors []string)
	SchemaType() string
	SchemaID() string
	Schema() []byte
}

type schema struct {
	T  string // type
	S  string // schema
	U  string // url
	St string // struct
}

// ShouldAddValidator only adds validator logic for package local structs
func (s schema) ShouldAddValidator() bool {
	return !strings.Contains(s.St, ".")
}

func (s schema) SchemaURL() string {
	t, _, err := api.SchemaURLForType(s.T)
	if err != nil {
		panic(err)
	}

	return t
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
	c := exec.Command("goimports", "-w", file)
	out, err := c.CombinedOutput()
	if err != nil {
		log.Printf("goimports failed: %s", string(out))
	}

	c = exec.Command("go", "fmt", file)
	out, err = c.CombinedOutput()
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
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/server/advisory/v1/client_connect.json", St: "srvadvisory.ConnectEventMsgV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/server/advisory/v1/client_disconnect.json", St: "srvadvisory.DisconnectEventMsgV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/server/advisory/v1/account_connections.json", St: "srvadvisory.AccountConnectionsV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/server/metric/v1/service_latency.json", St: "srvmetric.ServiceLatencyV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/api_audit.json", St: "jsadvisory.JetStreamAPIAuditV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/max_deliver.json", St: "jsadvisory.ConsumerDeliveryExceededAdvisoryV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/terminated.json", St: "jsadvisory.JSConsumerDeliveryTerminatedAdvisoryV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/stream_action.json", St: "jsadvisory.JSStreamActionAdvisoryV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/consumer_action.json", St: "jsadvisory.JSConsumerActionAdvisoryV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/snapshot_create.json", St: "jsadvisory.JSSnapshotCreateAdvisoryV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/snapshot_complete.json", St: "jsadvisory.JSSnapshotCompleteAdvisoryV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/restore_create.json", St: "jsadvisory.JSRestoreCreateAdvisoryV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/advisory/v1/restore_complete.json", St: "jsadvisory.JSRestoreCompleteAdvisoryV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/metric/v1/consumer_ack.json", St: "jsmetric.ConsumerAckMetricV1"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_configuration.json", St: "ConsumerConfig"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_configuration.json", St: "StreamConfig"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_template_configuration.json", St: "StreamTemplateConfig"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/account_info_response.json", St: "JSApiAccountInfoResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_create_request.json", St: "JSApiConsumerCreateRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_create_response.json", St: "JSApiConsumerCreateResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_delete_response.json", St: "JSApiConsumerDeleteResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_info_response.json", St: "JSApiConsumerInfoResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_list_request.json", St: "JSApiConsumerListRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_list_response.json", St: "JSApiConsumerListResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_names_request.json", St: "JSApiConsumerNamesRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_names_response.json", St: "JSApiConsumerNamesResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/consumer_getnext_request.json", St: "JSApiConsumerGetNextRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_create_response.json", St: "JSApiStreamCreateResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_delete_response.json", St: "JSApiStreamDeleteResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_info_response.json", St: "JSApiStreamInfoResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_list_request.json", St: "JSApiStreamListRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_list_response.json", St: "JSApiStreamListResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_msg_delete_response.json", St: "JSApiMsgDeleteResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_msg_get_request.json", St: "JSApiMsgGetRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_msg_get_response.json", St: "JSApiMsgGetResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_names_request.json", St: "JSApiStreamNamesRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_names_response.json", St: "JSApiStreamNamesResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_purge_response.json", St: "JSApiStreamPurgeResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_snapshot_response.json", St: "JSApiStreamSnapshotResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_snapshot_request.json", St: "JSApiStreamSnapshotRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_restore_response.json", St: "JSApiStreamRestoreResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_template_create_response.json", St: "JSApiStreamTemplateCreateResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_template_delete_response.json", St: "JSApiStreamTemplateDeleteResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_template_info_response.json", St: "JSApiStreamTemplateInfoResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_template_names_response.json", St: "JSApiStreamTemplateNamesResponse"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_template_names_request.json", St: "JSApiStreamTemplateNamesRequest"},
		&schema{U: "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas/jetstream/api/v1/stream_update_response.json", St: "JSApiStreamUpdateResponse"},
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
