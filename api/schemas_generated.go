// auto generated 2025-07-31 10:15:35.804828 +0300 EEST m=+0.007742626

package api

import (
	jsadvisory "github.com/nats-io/jsm.go/api/jetstream/advisory"
	jsmetric "github.com/nats-io/jsm.go/api/jetstream/metric"
	srvadvisory "github.com/nats-io/jsm.go/api/server/advisory"
	srvmetric "github.com/nats-io/jsm.go/api/server/metric"
	"github.com/nats-io/jsm.go/api/server/zmonitor"
	scfs "github.com/nats-io/jsm.go/schemas"
	"github.com/nats-io/nats.go/micro"
)

var schemaTypes = map[string]func() any{
	"io.nats.jetstream.advisory.v1.api_audit":                    func() any { return &jsadvisory.JetStreamAPIAuditV1{} },
	"io.nats.jetstream.advisory.v1.consumer_action":              func() any { return &jsadvisory.JSConsumerActionAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.consumer_group_pinned":        func() any { return &jsadvisory.JSConsumerGroupPinnedAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.consumer_group_unpinned":      func() any { return &jsadvisory.JSConsumerGroupUnPinnedAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.consumer_leader_elected":      func() any { return &jsadvisory.JSConsumerLeaderElectedV1{} },
	"io.nats.jetstream.advisory.v1.consumer_pause":               func() any { return &jsadvisory.JSConsumerPauseAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.consumer_quorum_lost":         func() any { return &jsadvisory.JSConsumerQuorumLostV1{} },
	"io.nats.jetstream.advisory.v1.domain_leader_elected":        func() any { return &jsadvisory.JSDomainLeaderElectedV1{} },
	"io.nats.jetstream.advisory.v1.max_deliver":                  func() any { return &jsadvisory.ConsumerDeliveryExceededAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.nak":                          func() any { return &jsadvisory.JSConsumerDeliveryNakAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.restore_complete":             func() any { return &jsadvisory.JSRestoreCompleteAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.restore_create":               func() any { return &jsadvisory.JSRestoreCreateAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.server_out_of_space":          func() any { return &jsadvisory.JSServerOutOfSpaceAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.server_removed":               func() any { return &jsadvisory.JSServerRemovedAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.snapshot_complete":            func() any { return &jsadvisory.JSSnapshotCompleteAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.snapshot_create":              func() any { return &jsadvisory.JSSnapshotCreateAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.stream_action":                func() any { return &jsadvisory.JSStreamActionAdvisoryV1{} },
	"io.nats.jetstream.advisory.v1.stream_leader_elected":        func() any { return &jsadvisory.JSStreamLeaderElectedV1{} },
	"io.nats.jetstream.advisory.v1.stream_quorum_lost":           func() any { return &jsadvisory.JSStreamQuorumLostV1{} },
	"io.nats.jetstream.advisory.v1.terminated":                   func() any { return &jsadvisory.JSConsumerDeliveryTerminatedAdvisoryV1{} },
	"io.nats.jetstream.api.v1.account_info_response":             func() any { return &JSApiAccountInfoResponse{} },
	"io.nats.jetstream.api.v1.account_purge_response":            func() any { return &JSApiAccountPurgeResponse{} },
	"io.nats.jetstream.api.v1.consumer_configuration":            func() any { return &ConsumerConfig{} },
	"io.nats.jetstream.api.v1.consumer_create_request":           func() any { return &JSApiConsumerCreateRequest{} },
	"io.nats.jetstream.api.v1.consumer_create_response":          func() any { return &JSApiConsumerCreateResponse{} },
	"io.nats.jetstream.api.v1.consumer_delete_response":          func() any { return &JSApiConsumerDeleteResponse{} },
	"io.nats.jetstream.api.v1.consumer_getnext_request":          func() any { return &JSApiConsumerGetNextRequest{} },
	"io.nats.jetstream.api.v1.consumer_info_response":            func() any { return &JSApiConsumerInfoResponse{} },
	"io.nats.jetstream.api.v1.consumer_leader_stepdown_response": func() any { return &JSApiConsumerLeaderStepDownResponse{} },
	"io.nats.jetstream.api.v1.consumer_list_request":             func() any { return &JSApiConsumerListRequest{} },
	"io.nats.jetstream.api.v1.consumer_list_response":            func() any { return &JSApiConsumerListResponse{} },
	"io.nats.jetstream.api.v1.consumer_names_request":            func() any { return &JSApiConsumerNamesRequest{} },
	"io.nats.jetstream.api.v1.consumer_names_response":           func() any { return &JSApiConsumerNamesResponse{} },
	"io.nats.jetstream.api.v1.consumer_pause_request":            func() any { return &JSApiConsumerPauseRequest{} },
	"io.nats.jetstream.api.v1.consumer_pause_response":           func() any { return &JSApiConsumerPauseResponse{} },
	"io.nats.jetstream.api.v1.consumer_unpin_request":            func() any { return &JSApiConsumerUnpinRequest{} },
	"io.nats.jetstream.api.v1.consumer_unpin_response":           func() any { return &JSApiConsumerUnpinResponse{} },
	"io.nats.jetstream.api.v1.meta_leader_stepdown_request":      func() any { return &JSApiLeaderStepDownRequest{} },
	"io.nats.jetstream.api.v1.meta_leader_stepdown_response":     func() any { return &JSApiLeaderStepDownResponse{} },
	"io.nats.jetstream.api.v1.meta_server_remove_request":        func() any { return &JSApiMetaServerRemoveRequest{} },
	"io.nats.jetstream.api.v1.meta_server_remove_response":       func() any { return &JSApiMetaServerRemoveResponse{} },
	"io.nats.jetstream.api.v1.pub_ack_response":                  func() any { return &JSPubAckResponse{} },
	"io.nats.jetstream.api.v1.stream_configuration":              func() any { return &StreamConfig{} },
	"io.nats.jetstream.api.v1.stream_create_request":             func() any { return &JSApiStreamCreateRequest{} },
	"io.nats.jetstream.api.v1.stream_create_response":            func() any { return &JSApiStreamCreateResponse{} },
	"io.nats.jetstream.api.v1.stream_delete_response":            func() any { return &JSApiStreamDeleteResponse{} },
	"io.nats.jetstream.api.v1.stream_info_request":               func() any { return &JSApiStreamInfoRequest{} },
	"io.nats.jetstream.api.v1.stream_info_response":              func() any { return &JSApiStreamInfoResponse{} },
	"io.nats.jetstream.api.v1.stream_leader_stepdown_response":   func() any { return &JSApiStreamLeaderStepDownResponse{} },
	"io.nats.jetstream.api.v1.stream_list_request":               func() any { return &JSApiStreamListRequest{} },
	"io.nats.jetstream.api.v1.stream_list_response":              func() any { return &JSApiStreamListResponse{} },
	"io.nats.jetstream.api.v1.stream_msg_delete_response":        func() any { return &JSApiMsgDeleteResponse{} },
	"io.nats.jetstream.api.v1.stream_msg_get_request":            func() any { return &JSApiMsgGetRequest{} },
	"io.nats.jetstream.api.v1.stream_msg_get_response":           func() any { return &JSApiMsgGetResponse{} },
	"io.nats.jetstream.api.v1.stream_names_request":              func() any { return &JSApiStreamNamesRequest{} },
	"io.nats.jetstream.api.v1.stream_names_response":             func() any { return &JSApiStreamNamesResponse{} },
	"io.nats.jetstream.api.v1.stream_purge_request":              func() any { return &JSApiStreamPurgeRequest{} },
	"io.nats.jetstream.api.v1.stream_purge_response":             func() any { return &JSApiStreamPurgeResponse{} },
	"io.nats.jetstream.api.v1.stream_remove_peer_request":        func() any { return &JSApiStreamRemovePeerRequest{} },
	"io.nats.jetstream.api.v1.stream_remove_peer_response":       func() any { return &JSApiStreamRemovePeerResponse{} },
	"io.nats.jetstream.api.v1.stream_restore_request":            func() any { return &JSApiStreamRestoreRequest{} },
	"io.nats.jetstream.api.v1.stream_restore_response":           func() any { return &JSApiStreamRestoreResponse{} },
	"io.nats.jetstream.api.v1.stream_snapshot_request":           func() any { return &JSApiStreamSnapshotRequest{} },
	"io.nats.jetstream.api.v1.stream_snapshot_response":          func() any { return &JSApiStreamSnapshotResponse{} },
	"io.nats.jetstream.api.v1.stream_update_response":            func() any { return &JSApiStreamUpdateResponse{} },
	"io.nats.jetstream.metric.v1.consumer_ack":                   func() any { return &jsmetric.ConsumerAckMetricV1{} },
	"io.nats.micro.v1.info_response":                             func() any { return &micro.Info{} },
	"io.nats.micro.v1.ping_response":                             func() any { return &micro.Ping{} },
	"io.nats.micro.v1.stats_response":                            func() any { return &micro.Stats{} },
	"io.nats.server.advisory.v1.account_connections":             func() any { return &srvadvisory.AccountConnectionsV1{} },
	"io.nats.server.advisory.v1.client_connect":                  func() any { return &srvadvisory.ConnectEventMsgV1{} },
	"io.nats.server.advisory.v1.client_disconnect":               func() any { return &srvadvisory.DisconnectEventMsgV1{} },
	"io.nats.server.metric.v1.service_latency":                   func() any { return &srvmetric.ServiceLatencyV1{} },
	"io.nats.server.monitor.v1.varz":                             func() any { return &zmonitor.VarzV1{} },
	"io.nats.unknown_message":                                    func() any { return &UnknownMessage{} },
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiAccountInfoResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.account_info_response
func (t JSApiAccountInfoResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.account_info_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiAccountInfoResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/account_info_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiAccountInfoResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiAccountPurgeResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.account_purge_response
func (t JSApiAccountPurgeResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.account_purge_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiAccountPurgeResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/account_purge_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiAccountPurgeResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t ConsumerConfig) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_configuration
func (t ConsumerConfig) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_configuration"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t ConsumerConfig) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_configuration.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t ConsumerConfig) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerCreateRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_create_request
func (t JSApiConsumerCreateRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_create_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerCreateRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_create_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerCreateRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerCreateResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_create_response
func (t JSApiConsumerCreateResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_create_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerCreateResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_create_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerCreateResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerDeleteResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_delete_response
func (t JSApiConsumerDeleteResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_delete_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerDeleteResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_delete_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerDeleteResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerGetNextRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_getnext_request
func (t JSApiConsumerGetNextRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_getnext_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerGetNextRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_getnext_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerGetNextRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerInfoResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_info_response
func (t JSApiConsumerInfoResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_info_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerInfoResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_info_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerInfoResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerLeaderStepDownResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_leader_stepdown_response
func (t JSApiConsumerLeaderStepDownResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_leader_stepdown_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerLeaderStepDownResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_leader_stepdown_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerLeaderStepDownResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerListRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_list_request
func (t JSApiConsumerListRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_list_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerListRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_list_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerListRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerListResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_list_response
func (t JSApiConsumerListResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_list_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerListResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_list_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerListResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerNamesRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_names_request
func (t JSApiConsumerNamesRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_names_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerNamesRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_names_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerNamesRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerNamesResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_names_response
func (t JSApiConsumerNamesResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_names_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerNamesResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_names_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerNamesResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerPauseRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_pause_request
func (t JSApiConsumerPauseRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_pause_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerPauseRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_pause_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerPauseRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerPauseResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_pause_response
func (t JSApiConsumerPauseResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_pause_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerPauseResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_pause_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerPauseResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerUnpinRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_unpin_request
func (t JSApiConsumerUnpinRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_unpin_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerUnpinRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_unpin_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerUnpinRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerUnpinResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_unpin_response
func (t JSApiConsumerUnpinResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_unpin_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerUnpinResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_unpin_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerUnpinResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiLeaderStepDownRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.meta_leader_stepdown_request
func (t JSApiLeaderStepDownRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.meta_leader_stepdown_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiLeaderStepDownRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/meta_leader_stepdown_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiLeaderStepDownRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiLeaderStepDownResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.meta_leader_stepdown_response
func (t JSApiLeaderStepDownResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.meta_leader_stepdown_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiLeaderStepDownResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/meta_leader_stepdown_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiLeaderStepDownResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMetaServerRemoveRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.meta_server_remove_request
func (t JSApiMetaServerRemoveRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.meta_server_remove_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiMetaServerRemoveRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/meta_server_remove_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiMetaServerRemoveRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMetaServerRemoveResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.meta_server_remove_response
func (t JSApiMetaServerRemoveResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.meta_server_remove_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiMetaServerRemoveResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/meta_server_remove_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiMetaServerRemoveResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSPubAckResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.pub_ack_response
func (t JSPubAckResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.pub_ack_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSPubAckResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/pub_ack_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSPubAckResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t StreamConfig) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_configuration
func (t StreamConfig) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_configuration"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t StreamConfig) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_configuration.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t StreamConfig) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamCreateRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_create_request
func (t JSApiStreamCreateRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_create_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamCreateRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_create_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamCreateRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamCreateResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_create_response
func (t JSApiStreamCreateResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_create_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamCreateResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_create_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamCreateResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamDeleteResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_delete_response
func (t JSApiStreamDeleteResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_delete_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamDeleteResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_delete_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamDeleteResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamInfoRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_info_request
func (t JSApiStreamInfoRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_info_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamInfoRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_info_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamInfoRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamInfoResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_info_response
func (t JSApiStreamInfoResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_info_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamInfoResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_info_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamInfoResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamLeaderStepDownResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_leader_stepdown_response
func (t JSApiStreamLeaderStepDownResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_leader_stepdown_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamLeaderStepDownResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_leader_stepdown_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamLeaderStepDownResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamListRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_list_request
func (t JSApiStreamListRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_list_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamListRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_list_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamListRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamListResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_list_response
func (t JSApiStreamListResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_list_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamListResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_list_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamListResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMsgDeleteResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_msg_delete_response
func (t JSApiMsgDeleteResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_msg_delete_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiMsgDeleteResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_msg_delete_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiMsgDeleteResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMsgGetRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_msg_get_request
func (t JSApiMsgGetRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_msg_get_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiMsgGetRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_msg_get_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiMsgGetRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMsgGetResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_msg_get_response
func (t JSApiMsgGetResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_msg_get_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiMsgGetResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_msg_get_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiMsgGetResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamNamesRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_names_request
func (t JSApiStreamNamesRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_names_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamNamesRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_names_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamNamesRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamNamesResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_names_response
func (t JSApiStreamNamesResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_names_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamNamesResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_names_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamNamesResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamPurgeRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_purge_request
func (t JSApiStreamPurgeRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_purge_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamPurgeRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_purge_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamPurgeRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamPurgeResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_purge_response
func (t JSApiStreamPurgeResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_purge_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamPurgeResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_purge_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamPurgeResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamRemovePeerRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_remove_peer_request
func (t JSApiStreamRemovePeerRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_remove_peer_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamRemovePeerRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_remove_peer_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamRemovePeerRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamRemovePeerResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_remove_peer_response
func (t JSApiStreamRemovePeerResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_remove_peer_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamRemovePeerResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_remove_peer_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamRemovePeerResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamRestoreRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_restore_request
func (t JSApiStreamRestoreRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_restore_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamRestoreRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_restore_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamRestoreRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamRestoreResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_restore_response
func (t JSApiStreamRestoreResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_restore_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamRestoreResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_restore_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamRestoreResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamSnapshotRequest) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_snapshot_request
func (t JSApiStreamSnapshotRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_snapshot_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamSnapshotRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_snapshot_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamSnapshotRequest) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamSnapshotResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_snapshot_response
func (t JSApiStreamSnapshotResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_snapshot_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamSnapshotResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_snapshot_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamSnapshotResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamUpdateResponse) Validate(v ...StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_update_response
func (t JSApiStreamUpdateResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_update_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamUpdateResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_update_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamUpdateResponse) Schema() ([]byte, error) {
	f, err := SchemaFileForType(t.SchemaType())
	if err != nil {
		return nil, err
	}
	return scfs.Load(f)
}
