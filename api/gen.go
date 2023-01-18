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

//go:build ignore
// +build ignore

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/nats-io/jsm.go/api"
	scfs "github.com/nats-io/jsm.go/schemas"
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
    scfs "github.com/nats-io/jsm.go/schemas"
	"github.com/nats-io/nats.go/micro"
)

var schemaTypes = map[string]func() any {
{{- range . }}
{{- if .St }}
    "{{ .T }}": func() any { return &{{ .St }}{} },
{{- end }}
{{- end }}
	"io.nats.unknown_message": func() any { return &UnknownMessage{} },
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
func (t {{ .St }}) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

{{- end }}
{{- end }}
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
	P  string // path
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

func fetchErrors() error {
	resp, err := http.Get("https://raw.githubusercontent.com/nats-io/nats-server/main/server/errors.json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create("schemas/server/errors.json")
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func getSchema(u string) (title string, id string, body string, err error) {
	log.Printf("Fetching %s", u)

	f, err := api.SchemaFileForType("io.nats." + strings.TrimSuffix(strings.ReplaceAll(u, "/", "."), ".json"))
	panicIfErr(err)
	data, err := scfs.Load(f)
	panicIfErr(err)

	idt := &idDetect{}
	err = json.Unmarshal(data, idt)
	panicIfErr(err)

	log.Printf("Detected %+v", *idt)
	return idt.Title, idt.ID, base64.StdEncoding.EncodeToString(data), nil
}

func main() {
	s := schemas{
		&schema{P: "server/advisory/v1/client_connect.json", St: "srvadvisory.ConnectEventMsgV1"},
		&schema{P: "server/advisory/v1/client_disconnect.json", St: "srvadvisory.DisconnectEventMsgV1"},
		&schema{P: "server/advisory/v1/account_connections.json", St: "srvadvisory.AccountConnectionsV1"},
		&schema{P: "server/metric/v1/service_latency.json", St: "srvmetric.ServiceLatencyV1"},
		&schema{P: "jetstream/advisory/v1/api_audit.json", St: "jsadvisory.JetStreamAPIAuditV1"},
		&schema{P: "jetstream/advisory/v1/max_deliver.json", St: "jsadvisory.ConsumerDeliveryExceededAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/nak.json", St: "jsadvisory.JSConsumerDeliveryNakAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/terminated.json", St: "jsadvisory.JSConsumerDeliveryTerminatedAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/stream_action.json", St: "jsadvisory.JSStreamActionAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/consumer_action.json", St: "jsadvisory.JSConsumerActionAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/snapshot_create.json", St: "jsadvisory.JSSnapshotCreateAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/snapshot_complete.json", St: "jsadvisory.JSSnapshotCompleteAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/restore_create.json", St: "jsadvisory.JSRestoreCreateAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/restore_complete.json", St: "jsadvisory.JSRestoreCompleteAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/stream_leader_elected.json", St: "jsadvisory.JSStreamLeaderElectedV1"},
		&schema{P: "jetstream/advisory/v1/consumer_leader_elected.json", St: "jsadvisory.JSConsumerLeaderElectedV1"},
		&schema{P: "jetstream/advisory/v1/stream_quorum_lost.json", St: "jsadvisory.JSStreamQuorumLostV1"},
		&schema{P: "jetstream/advisory/v1/consumer_quorum_lost.json", St: "jsadvisory.JSConsumerQuorumLostV1"},
		&schema{P: "jetstream/advisory/v1/server_out_of_space.json", St: "jsadvisory.JSServerOutOfSpaceAdvisoryV1"},
		&schema{P: "jetstream/metric/v1/consumer_ack.json", St: "jsmetric.ConsumerAckMetricV1"},
		&schema{P: "jetstream/api/v1/consumer_configuration.json", St: "ConsumerConfig"},
		&schema{P: "jetstream/api/v1/stream_configuration.json", St: "StreamConfig"},
		&schema{P: "jetstream/api/v1/stream_template_configuration.json", St: "StreamTemplateConfig"},
		&schema{P: "jetstream/api/v1/account_info_response.json", St: "JSApiAccountInfoResponse"},
		&schema{P: "jetstream/api/v1/consumer_create_request.json", St: "JSApiConsumerCreateRequest"},
		&schema{P: "jetstream/api/v1/consumer_create_response.json", St: "JSApiConsumerCreateResponse"},
		&schema{P: "jetstream/api/v1/consumer_delete_response.json", St: "JSApiConsumerDeleteResponse"},
		&schema{P: "jetstream/api/v1/consumer_info_response.json", St: "JSApiConsumerInfoResponse"},
		&schema{P: "jetstream/api/v1/consumer_list_request.json", St: "JSApiConsumerListRequest"},
		&schema{P: "jetstream/api/v1/consumer_list_response.json", St: "JSApiConsumerListResponse"},
		&schema{P: "jetstream/api/v1/consumer_names_request.json", St: "JSApiConsumerNamesRequest"},
		&schema{P: "jetstream/api/v1/consumer_names_response.json", St: "JSApiConsumerNamesResponse"},
		&schema{P: "jetstream/api/v1/consumer_getnext_request.json", St: "JSApiConsumerGetNextRequest"},
		&schema{P: "jetstream/api/v1/consumer_leader_stepdown_response.json", St: "JSApiConsumerLeaderStepDownResponse"},
		&schema{P: "jetstream/api/v1/stream_create_request.json", St: "JSApiStreamCreateRequest"},
		&schema{P: "jetstream/api/v1/stream_create_response.json", St: "JSApiStreamCreateResponse"},
		&schema{P: "jetstream/api/v1/stream_delete_response.json", St: "JSApiStreamDeleteResponse"},
		&schema{P: "jetstream/api/v1/stream_info_request.json", St: "JSApiStreamInfoRequest"},
		&schema{P: "jetstream/api/v1/stream_info_response.json", St: "JSApiStreamInfoResponse"},
		&schema{P: "jetstream/api/v1/stream_list_request.json", St: "JSApiStreamListRequest"},
		&schema{P: "jetstream/api/v1/stream_list_response.json", St: "JSApiStreamListResponse"},
		&schema{P: "jetstream/api/v1/stream_msg_delete_response.json", St: "JSApiMsgDeleteResponse"},
		&schema{P: "jetstream/api/v1/stream_msg_get_request.json", St: "JSApiMsgGetRequest"},
		&schema{P: "jetstream/api/v1/stream_msg_get_response.json", St: "JSApiMsgGetResponse"},
		&schema{P: "jetstream/api/v1/stream_names_request.json", St: "JSApiStreamNamesRequest"},
		&schema{P: "jetstream/api/v1/stream_names_response.json", St: "JSApiStreamNamesResponse"},
		&schema{P: "jetstream/api/v1/stream_purge_request.json", St: "JSApiStreamPurgeRequest"},
		&schema{P: "jetstream/api/v1/stream_purge_response.json", St: "JSApiStreamPurgeResponse"},
		&schema{P: "jetstream/api/v1/stream_snapshot_response.json", St: "JSApiStreamSnapshotResponse"},
		&schema{P: "jetstream/api/v1/stream_snapshot_request.json", St: "JSApiStreamSnapshotRequest"},
		&schema{P: "jetstream/api/v1/stream_restore_request.json", St: "JSApiStreamRestoreRequest"},
		&schema{P: "jetstream/api/v1/stream_restore_response.json", St: "JSApiStreamRestoreResponse"},
		&schema{P: "jetstream/api/v1/stream_template_create_request.json", St: "JSApiStreamTemplateCreateRequest"},
		&schema{P: "jetstream/api/v1/stream_template_create_response.json", St: "JSApiStreamTemplateCreateResponse"},
		&schema{P: "jetstream/api/v1/stream_template_delete_response.json", St: "JSApiStreamTemplateDeleteResponse"},
		&schema{P: "jetstream/api/v1/stream_template_info_response.json", St: "JSApiStreamTemplateInfoResponse"},
		&schema{P: "jetstream/api/v1/stream_template_names_response.json", St: "JSApiStreamTemplateNamesResponse"},
		&schema{P: "jetstream/api/v1/stream_template_names_request.json", St: "JSApiStreamTemplateNamesRequest"},
		&schema{P: "jetstream/api/v1/stream_update_response.json", St: "JSApiStreamUpdateResponse"},
		&schema{P: "jetstream/api/v1/stream_remove_peer_request.json", St: "JSApiStreamRemovePeerRequest"},
		&schema{P: "jetstream/api/v1/stream_remove_peer_response.json", St: "JSApiStreamRemovePeerResponse"},
		&schema{P: "jetstream/api/v1/stream_leader_stepdown_response.json", St: "JSApiStreamLeaderStepDownResponse"},
		&schema{P: "jetstream/api/v1/pub_ack_response.json", St: "JSPubAckResponse"},
		&schema{P: "jetstream/api/v1/meta_leader_stepdown_request.json", St: "JSApiLeaderStepDownRequest"},
		&schema{P: "jetstream/api/v1/meta_leader_stepdown_response.json", St: "JSApiLeaderStepDownResponse"},
		&schema{P: "jetstream/api/v1/meta_server_remove_request.json", St: "JSApiMetaServerRemoveRequest"},
		&schema{P: "jetstream/api/v1/meta_server_remove_response.json", St: "JSApiMetaServerRemoveResponse"},
		&schema{P: "jetstream/api/v1/account_purge_response.json", St: "JSApiAccountPurgeResponse"},
		&schema{P: "micro/v1/info_response.json", St: "micro.Info"},
		&schema{P: "micro/v1/ping_response.json", St: "micro.Ping"},
		&schema{P: "micro/v1/stats_response.json", St: "micro.Stats"},
		&schema{P: "micro/v1/schema_response.json", St: "micro.SchemaResp"},
	}

	for _, i := range s {
		title, _, body, err := getSchema(i.P)
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

	panicIfErr(fetchErrors())
}
