//go:build ignore

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/nats-io/jsm.go/registry"
	scfs "github.com/nats-io/jsm.go/schemas"
)

type schema struct {
	T   string // type
	S   string // schema
	P   string // path
	St  string // struct
	Req string // request subject
	Res string // response subject
	W   string // wildcard subject
	F   string // format subject
}

func (s schema) SchemaURL() string {
	t, _, err := registry.SchemaURLForType(s.T)
	if err != nil {
		panic(err)
	}

	return t
}

type templateEnv struct {
	Schemas schemas
	Package string
	Imports []string
}

type idDetect struct {
	ID    string `json:"$id"`
	Title string `json:"title"`
}

type schemas []*schema

func main() {
	// register all types, response subjects, request subjects and factories
	renderSchema("registry/micro_gen.go", "registry", registerFileTemplate, populateSchemas(microSchemas()), "github.com/nats-io/nats.go/micro")
	renderSchema("registry/api_gen.go", "registry", registerFileTemplate, populateSchemas(jsApiSchemas()))
	renderSchema("registry/js_metric_gen.go", "registry", registerFileTemplate, populateSchemas(jsMetricSchemas()), "github.com/nats-io/jsm.go/api/jetstream/metric")
	renderSchema("registry/js_advisory_gen.go", "registry", registerFileTemplate, populateSchemas(jsAdvisorySchemas()), "github.com/nats-io/jsm.go/api/jetstream/advisory")
	renderSchema("registry/server_metric_gen.go", "registry", registerFileTemplate, populateSchemas(serverMetricSchemas()), "github.com/nats-io/jsm.go/api/server/metric")
	renderSchema("registry/server_advisory_gen.go", "registry", registerFileTemplate, populateSchemas(serverAdvisorySchemas()), "github.com/nats-io/jsm.go/api/server/advisory")
	renderSchema("registry/server_zmonitor_gen.go", "registry", registerFileTemplate, populateSchemas(zMonitorSchemas()), "github.com/nats-io/jsm.go/api/server/zmonitor")

	// add schema validation functions
	renderSchema("api/registry_gen.go", "api", validatorFunctionsTemplate, populateSchemas(jsApiSchemas()))
	renderSchema("api/jetstream/advisory/registry_gen.go", "advisory", validatorFunctionsTemplate, populateSchemas(jsAdvisorySchemas()))
	renderSchema("api/jetstream/metric/registry_gen.go", "metric", validatorFunctionsTemplate, populateSchemas(jsMetricSchemas()))
	renderSchema("api/server/advisory/registry_gen.go", "advisory", validatorFunctionsTemplate, populateSchemas(serverAdvisorySchemas()))
	renderSchema("api/server/metric/registry_gen.go", "metric", validatorFunctionsTemplate, populateSchemas(serverMetricSchemas()))
	renderSchema("api/server/zmonitor/registry_gen.go", "zmonitor", validatorFunctionsTemplate, populateSchemas(zMonitorSchemas()))
}

var validatorFunctionsTemplate = `// auto generated {{Now}}
package {{.Package}}

import (
	"github.com/nats-io/jsm.go/registry/validator"
	scfs "github.com/nats-io/jsm.go/schemas"
)

{{- range .Schemas }}
// Validate performs a JSON Schema validation of the configuration
func (t {{ .St | StripPackage }}) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type {{ .T }}
func (t {{ .St | StripPackage }}) SchemaType() string {
	return "{{ .T }}"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t {{ .St | StripPackage }}) SchemaID() string {
	return "{{ .SchemaURL }}"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t {{ .St | StripPackage }}) Schema() ([]byte, error) {
	return scfs.Load("{{ SchemaFileForType .T }}")
}

{{- if .Req }}
// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t {{ .St | StripPackage }}) ApiSubjectPattern() (string, error) {
	return {{ .W | StripPackage }}, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t {{ .St | StripPackage }}) ApiSubjectFormat() (string, error) {
	return {{ .F | StripPackage }}, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t {{ .St | StripPackage }}) ApiSubjectPrefix() (string, error) {
	return {{ .Req | StripPackage }}, nil
}
{{- end }}
{{- end }}
`

var registerFileTemplate = `// auto generated {{Now}}
{{ $pkg := .Package }}
package {{.Package}}

import (
{{- range .Imports}}
  "{{ . }}"
{{- end }}
{{- if ne .Package  "registry" }}
  "github.com/nats-io/jsm.go/registry"
{{- end }}
)

func init() {
{{- range .Schemas }}
	{{ RegistryPrefix $pkg }}RegisterTypeFactory("{{ .T }}", func() any { return &{{ StripPackageIfNotRegistry $pkg .St }}{} })
{{- if .Req }}
    {{ RegistryPrefix $pkg }}RegisterRequestSubjectType({{ StripPackageIfNotRegistry $pkg .Req }}, "{{ .T }}")
{{- end }}
{{- if .Res }}
    {{ RegistryPrefix $pkg }}RegisterResponseSubjectType({{ StripPackageIfNotRegistry $pkg .Res }}, "{{ .T }}")
{{- end }}
{{- if .W }}
    {{ RegistryPrefix $pkg }}RegisterWildcardType({{ StripPackageIfNotRegistry $pkg .W }}, "{{ .T }}")
{{- end }}
{{- end }}
}
`

func renderSchema(target string, pkg string, tmpl string, s schemas, imports ...string) {
	fmt.Printf("Generating registry helpers in %q for package %q\n", target, pkg)
	funcMap := template.FuncMap{
		"SchemaFileForType": func(t string) string {
			sch, _ := registry.SchemaFileForType(t)
			return sch
		},
		"Now": func() string { return fmt.Sprintf("%s", time.Now()) },
		"RegistryPrefix": func(p string) string {
			if p == "registry" {
				return ""
			}

			return "registry."
		},
		"StripPackage": func(p string) string {
			parts := strings.SplitN(p, ".", 2)
			if len(parts) != 2 {
				return p
			}

			return parts[1]
		},
		"StripPackageIfNotRegistry": func(pkg string, p string) string {
			if pkg == "registry" || pkg == "loaders" {
				return p
			}

			parts := strings.SplitN(p, ".", 2)
			if len(parts) != 2 {
				return p
			}

			return parts[1]
		},
	}
	t, err := template.New("schemas").Funcs(funcMap).Parse(tmpl)
	panicIfErr(err)

	out, err := os.Create(target)
	panicIfErr(err)

	err = t.Execute(out, templateEnv{
		Schemas: s,
		Package: pkg,
		Imports: imports,
	})
	panicIfErr(err)

	out.Close()
	err = goFmt(out.Name())
	panicIfErr(err)
}

func populateSchemas(s schemas) schemas {
	for _, i := range s {
		title, _, body, err := getSchema(i.P)
		panicIfErr(err)

		i.S = body
		if i.T == "" {
			i.T = title
		}

		if i.Req != "" {
			i.W = strings.TrimSuffix(i.Req, "Prefix")
			i.F = fmt.Sprintf("%s%s", i.W, "T")
		}
	}

	sort.Slice(s, func(i, j int) bool { return s[i].P < s[j].P })

	return s
}

func microSchemas() schemas {
	s := schemas{
		&schema{P: "micro/v1/info_response.json", St: "micro.Info"},
		&schema{P: "micro/v1/ping_response.json", St: "micro.Ping"},
		&schema{P: "micro/v1/stats_response.json", St: "micro.Stats"},
	}

	return populateSchemas(s)
}

func jsMetricSchemas() schemas {
	s := schemas{
		&schema{P: "jetstream/metric/v1/consumer_ack.json", St: "metric.ConsumerAckMetricV1"},
	}

	return populateSchemas(s)
}

func serverMetricSchemas() schemas {
	s := schemas{
		&schema{P: "server/metric/v1/service_latency.json", St: "metric.ServiceLatencyV1"},
	}

	return populateSchemas(s)
}

func serverAdvisorySchemas() schemas {
	s := schemas{
		&schema{P: "server/advisory/v1/account_connections.json", St: "advisory.AccountConnectionsV1"},
		&schema{P: "server/advisory/v1/client_connect.json", St: "advisory.ConnectEventMsgV1"},
		&schema{P: "server/advisory/v1/client_disconnect.json", St: "advisory.DisconnectEventMsgV1"},
	}

	return populateSchemas(s)
}

func zMonitorSchemas() schemas {
	s := schemas{
		&schema{P: "server/monitor/v1/varz.json", St: "zmonitor.VarzV1"},
	}

	return populateSchemas(s)
}

func jsAdvisorySchemas() schemas {
	s := schemas{
		&schema{P: "jetstream/advisory/v1/api_audit.json", St: "advisory.JetStreamAPIAuditV1"},
		&schema{P: "jetstream/advisory/v1/stream_batch_abandoned.json", St: "advisory.JSStreamBatchAbandonedAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/consumer_action.json", St: "advisory.JSConsumerActionAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/consumer_group_pinned.json", St: "advisory.JSConsumerGroupPinnedAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/consumer_group_unpinned.json", St: "advisory.JSConsumerGroupUnPinnedAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/consumer_leader_elected.json", St: "advisory.JSConsumerLeaderElectedV1"},
		&schema{P: "jetstream/advisory/v1/consumer_pause.json", St: "advisory.JSConsumerPauseAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/consumer_quorum_lost.json", St: "advisory.JSConsumerQuorumLostV1"},
		&schema{P: "jetstream/advisory/v1/domain_leader_elected.json", St: "advisory.JSDomainLeaderElectedV1"},
		&schema{P: "jetstream/advisory/v1/max_deliver.json", St: "advisory.ConsumerDeliveryExceededAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/nak.json", St: "advisory.JSConsumerDeliveryNakAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/restore_complete.json", St: "advisory.JSRestoreCompleteAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/restore_create.json", St: "advisory.JSRestoreCreateAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/server_out_of_space.json", St: "advisory.JSServerOutOfSpaceAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/server_removed.json", St: "advisory.JSServerRemovedAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/snapshot_complete.json", St: "advisory.JSSnapshotCompleteAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/snapshot_create.json", St: "advisory.JSSnapshotCreateAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/stream_action.json", St: "advisory.JSStreamActionAdvisoryV1"},
		&schema{P: "jetstream/advisory/v1/stream_leader_elected.json", St: "advisory.JSStreamLeaderElectedV1"},
		&schema{P: "jetstream/advisory/v1/stream_quorum_lost.json", St: "advisory.JSStreamQuorumLostV1"},
		&schema{P: "jetstream/advisory/v1/terminated.json", St: "advisory.JSConsumerDeliveryTerminatedAdvisoryV1"},
	}

	return populateSchemas(s)
}

func jsApiSchemas() schemas {
	return schemas{
		&schema{P: "jetstream/api/v1/account_info_response.json", St: "api.JSApiAccountInfoResponse", Res: "api.JSApiAccountInfoPrefix"},
		&schema{P: "jetstream/api/v1/account_purge_response.json", St: "api.JSApiAccountPurgeResponse", Res: "api.JSApiAccountPurgePrefix"},
		&schema{P: "jetstream/api/v1/consumer_configuration.json", St: "api.ConsumerConfig"},
		&schema{P: "jetstream/api/v1/consumer_create_request.json", St: "api.JSApiConsumerCreateRequest", Req: "api.JSApiConsumerCreateWithNamePrefix"},
		&schema{P: "jetstream/api/v1/consumer_create_response.json", St: "api.JSApiConsumerCreateResponse", Res: "api.JSApiConsumerCreatePrefix"},
		&schema{P: "jetstream/api/v1/consumer_delete_response.json", St: "api.JSApiConsumerDeleteResponse", Res: "api.JSApiConsumerDeletePrefix"},
		&schema{P: "jetstream/api/v1/consumer_getnext_request.json", St: "api.JSApiConsumerGetNextRequest", Req: "api.JSApiRequestNextPrefix"},
		&schema{P: "jetstream/api/v1/consumer_info_request.json", St: "api.JSApiConsumerInfoRequest", Req: "api.JSApiConsumerInfoPrefix"},
		&schema{P: "jetstream/api/v1/consumer_info_response.json", St: "api.JSApiConsumerInfoResponse", Res: "api.JSApiConsumerInfoPrefix"},
		&schema{P: "jetstream/api/v1/consumer_leader_stepdown_request.json", St: "api.JSApiConsumerLeaderStepdownRequest", Req: "api.JSApiConsumerLeaderStepDownPrefix"},
		&schema{P: "jetstream/api/v1/consumer_leader_stepdown_response.json", St: "api.JSApiConsumerLeaderStepDownResponse", Res: "api.JSApiConsumerLeaderStepDownPrefix"},
		&schema{P: "jetstream/api/v1/consumer_list_request.json", St: "api.JSApiConsumerListRequest", Req: "api.JSApiConsumerListPrefix"},
		&schema{P: "jetstream/api/v1/consumer_list_response.json", St: "api.JSApiConsumerListResponse", Res: "api.JSApiConsumerListPrefix"},
		&schema{P: "jetstream/api/v1/consumer_names_request.json", St: "api.JSApiConsumerNamesRequest", Req: "api.JSApiConsumerNamesPrefix"},
		&schema{P: "jetstream/api/v1/consumer_names_response.json", St: "api.JSApiConsumerNamesResponse", Res: "api.JSApiConsumerNamesPrefix"},
		&schema{P: "jetstream/api/v1/consumer_reset_request.json", St: "api.JSApiConsumerResetRequest", Req: "api.JSApiConsumerResetPrefix"},
		&schema{P: "jetstream/api/v1/consumer_reset_response.json", St: "api.JSApiConsumerResetResponse", Res: "api.JSApiConsumerResetPrefix"},
		&schema{P: "jetstream/api/v1/consumer_pause_request.json", St: "api.JSApiConsumerPauseRequest", Req: "api.JSApiConsumerPausePrefix"},
		&schema{P: "jetstream/api/v1/consumer_pause_response.json", St: "api.JSApiConsumerPauseResponse", Res: "api.JSApiConsumerPausePrefix"},
		&schema{P: "jetstream/api/v1/consumer_unpin_request.json", St: "api.JSApiConsumerUnpinRequest", Req: "api.JSApiConsumerUnpinPrefix"},
		&schema{P: "jetstream/api/v1/consumer_unpin_response.json", St: "api.JSApiConsumerUnpinResponse", Res: "api.JSApiConsumerUnpinPrefix"},
		&schema{P: "jetstream/api/v1/meta_leader_stepdown_request.json", St: "api.JSApiLeaderStepDownRequest", Req: "api.JSApiLeaderStepDownPrefix"},
		&schema{P: "jetstream/api/v1/meta_leader_stepdown_response.json", St: "api.JSApiLeaderStepDownResponse", Res: "api.JSApiLeaderStepDownPrefix"},
		&schema{P: "jetstream/api/v1/meta_server_remove_request.json", St: "api.JSApiMetaServerRemoveRequest", Req: "api.JSApiServerRemovePrefix"},
		&schema{P: "jetstream/api/v1/meta_server_remove_response.json", St: "api.JSApiMetaServerRemoveResponse", Res: "api.JSApiServerRemovePrefix"},
		&schema{P: "jetstream/api/v1/pub_ack_response.json", St: "api.JSPubAckResponse", Res: "api.JSAckPrefix"},
		&schema{P: "jetstream/api/v1/stream_configuration.json", St: "api.StreamConfig"},
		&schema{P: "jetstream/api/v1/stream_create_request.json", St: "api.JSApiStreamCreateRequest", Req: "api.JSApiStreamCreatePrefix"},
		&schema{P: "jetstream/api/v1/stream_create_response.json", St: "api.JSApiStreamCreateResponse", Res: "api.JSApiStreamCreatePrefix"},
		&schema{P: "jetstream/api/v1/stream_delete_response.json", St: "api.JSApiStreamDeleteResponse", Res: "api.JSApiStreamDeletePrefix"},
		&schema{P: "jetstream/api/v1/stream_info_request.json", St: "api.JSApiStreamInfoRequest", Req: "api.JSApiStreamInfoPrefix"},
		&schema{P: "jetstream/api/v1/stream_info_response.json", St: "api.JSApiStreamInfoResponse", Res: "api.JSApiStreamInfoPrefix"},
		&schema{P: "jetstream/api/v1/stream_leader_stepdown_request.json", St: "api.JSApiStreamLeaderStepDownRequest", Req: "api.JSApiStreamLeaderStepDownPrefix"},
		&schema{P: "jetstream/api/v1/stream_leader_stepdown_response.json", St: "api.JSApiStreamLeaderStepDownResponse", Res: "api.JSApiStreamLeaderStepDownPrefix"},
		&schema{P: "jetstream/api/v1/stream_list_request.json", St: "api.JSApiStreamListRequest", Req: "api.JSApiStreamListPrefix"},
		&schema{P: "jetstream/api/v1/stream_list_response.json", St: "api.JSApiStreamListResponse", Res: "api.JSApiStreamListPrefix"},
		&schema{P: "jetstream/api/v1/stream_msg_delete_request.json", St: "api.JSApiMsgDeleteRequest", Req: "api.JSApiMsgDeletePrefix"},
		&schema{P: "jetstream/api/v1/stream_msg_delete_response.json", St: "api.JSApiMsgDeleteResponse", Res: "api.JSApiMsgDeletePrefix"},
		&schema{P: "jetstream/api/v1/stream_msg_get_request.json", St: "api.JSApiMsgGetRequest", Req: "api.JSApiMsgGetPrefix"},
		&schema{P: "jetstream/api/v1/stream_msg_get_response.json", St: "api.JSApiMsgGetResponse", Res: "api.JSApiMsgGetPrefix"},
		&schema{P: "jetstream/api/v1/stream_names_request.json", St: "api.JSApiStreamNamesRequest", Req: "api.JSApiStreamNamesPrefix"},
		&schema{P: "jetstream/api/v1/stream_names_response.json", St: "api.JSApiStreamNamesResponse", Res: "api.JSApiStreamNamesPrefix"},
		&schema{P: "jetstream/api/v1/stream_purge_request.json", St: "api.JSApiStreamPurgeRequest", Req: "api.JSApiStreamPurgePrefix"},
		&schema{P: "jetstream/api/v1/stream_purge_response.json", St: "api.JSApiStreamPurgeResponse", Res: "api.JSApiStreamPurgePrefix"},
		&schema{P: "jetstream/api/v1/stream_remove_peer_request.json", St: "api.JSApiStreamRemovePeerRequest", Req: "api.JSApiStreamRemovePeerPrefix"},
		&schema{P: "jetstream/api/v1/stream_remove_peer_response.json", St: "api.JSApiStreamRemovePeerResponse", Res: "api.JSApiStreamRemovePeerPrefix"},
		&schema{P: "jetstream/api/v1/stream_restore_request.json", St: "api.JSApiStreamRestoreRequest", Req: "api.JSApiStreamRestorePrefix"},
		&schema{P: "jetstream/api/v1/stream_restore_response.json", St: "api.JSApiStreamRestoreResponse", Res: "api.JSApiStreamRestorePrefix"},
		&schema{P: "jetstream/api/v1/stream_snapshot_request.json", St: "api.JSApiStreamSnapshotRequest", Req: "api.JSApiStreamSnapshotPrefix"},
		&schema{P: "jetstream/api/v1/stream_snapshot_response.json", St: "api.JSApiStreamSnapshotResponse", Res: "api.JSApiStreamSnapshotPrefix"},
		&schema{P: "jetstream/api/v1/stream_update_request.json", St: "api.JSApiStreamUpdateRequest", Req: "api.JSApiStreamUpdatePrefix"},
		&schema{P: "jetstream/api/v1/stream_update_response.json", St: "api.JSApiStreamUpdateResponse", Res: "api.JSApiStreamUpdatePrefix"},
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

func getSchema(u string) (title string, id string, body string, err error) {
	f, err := registry.SchemaFileForType("io.nats." + strings.TrimSuffix(strings.ReplaceAll(u, "/", "."), ".json"))
	panicIfErr(err)
	data, err := scfs.Load(f)
	panicIfErr(err)

	idt := &idDetect{}
	err = json.Unmarshal(data, idt)
	panicIfErr(err)

	return idt.Title, idt.ID, base64.StdEncoding.EncodeToString(data), nil
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
