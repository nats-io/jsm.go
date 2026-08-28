// auto generated 2026-08-28 10:23:33.102827 +0200 CEST m=+0.804575418
package api

import (
	"github.com/nats-io/jsm.go/registry/validator"
	scfs "github.com/nats-io/jsm.go/schemas"
)

// Validate performs a JSON Schema validation of the configuration
func (t JSApiAccountInfoResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/account_info_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiAccountPurgeResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/account_purge_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t ConsumerConfig) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_configuration.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerCreateRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_create_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerCreateRequest) ApiSubjectPattern() (string, error) {
	return JSApiConsumerCreateWithName, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerCreateRequest) ApiSubjectFormat() (string, error) {
	return JSApiConsumerCreateWithNameT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerCreateRequest) ApiSubjectPrefix() (string, error) {
	return JSApiConsumerCreateWithNamePrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerCreateResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_create_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerDeleteResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_delete_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerGetNextRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_getnext_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerGetNextRequest) ApiSubjectPattern() (string, error) {
	return JSApiRequestNext, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerGetNextRequest) ApiSubjectFormat() (string, error) {
	return JSApiRequestNextT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerGetNextRequest) ApiSubjectPrefix() (string, error) {
	return JSApiRequestNextPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerInfoRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_info_request
func (t JSApiConsumerInfoRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_info_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerInfoRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_info_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerInfoRequest) Schema() ([]byte, error) {
	return scfs.Load("jetstream/api/v1/consumer_info_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerInfoRequest) ApiSubjectPattern() (string, error) {
	return JSApiConsumerInfo, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerInfoRequest) ApiSubjectFormat() (string, error) {
	return JSApiConsumerInfoT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerInfoRequest) ApiSubjectPrefix() (string, error) {
	return JSApiConsumerInfoPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerInfoResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_info_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerLeaderStepdownRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_leader_stepdown_request
func (t JSApiConsumerLeaderStepdownRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_leader_stepdown_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerLeaderStepdownRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_leader_stepdown_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerLeaderStepdownRequest) Schema() ([]byte, error) {
	return scfs.Load("jetstream/api/v1/consumer_leader_stepdown_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerLeaderStepdownRequest) ApiSubjectPattern() (string, error) {
	return JSApiConsumerLeaderStepDown, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerLeaderStepdownRequest) ApiSubjectFormat() (string, error) {
	return JSApiConsumerLeaderStepDownT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerLeaderStepdownRequest) ApiSubjectPrefix() (string, error) {
	return JSApiConsumerLeaderStepDownPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerLeaderStepDownResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_leader_stepdown_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerListRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_list_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerListRequest) ApiSubjectPattern() (string, error) {
	return JSApiConsumerList, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerListRequest) ApiSubjectFormat() (string, error) {
	return JSApiConsumerListT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerListRequest) ApiSubjectPrefix() (string, error) {
	return JSApiConsumerListPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerListResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_list_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerNamesRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_names_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerNamesRequest) ApiSubjectPattern() (string, error) {
	return JSApiConsumerNames, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerNamesRequest) ApiSubjectFormat() (string, error) {
	return JSApiConsumerNamesT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerNamesRequest) ApiSubjectPrefix() (string, error) {
	return JSApiConsumerNamesPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerNamesResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_names_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerPauseRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_pause_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerPauseRequest) ApiSubjectPattern() (string, error) {
	return JSApiConsumerPause, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerPauseRequest) ApiSubjectFormat() (string, error) {
	return JSApiConsumerPauseT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerPauseRequest) ApiSubjectPrefix() (string, error) {
	return JSApiConsumerPausePrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerPauseResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_pause_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerResetRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_reset_request
func (t JSApiConsumerResetRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_reset_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerResetRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_reset_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerResetRequest) Schema() ([]byte, error) {
	return scfs.Load("jetstream/api/v1/consumer_reset_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerResetRequest) ApiSubjectPattern() (string, error) {
	return JSApiConsumerReset, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerResetRequest) ApiSubjectFormat() (string, error) {
	return JSApiConsumerResetT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerResetRequest) ApiSubjectPrefix() (string, error) {
	return JSApiConsumerResetPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerResetResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.consumer_reset_response
func (t JSApiConsumerResetResponse) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_reset_response"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiConsumerResetResponse) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/consumer_reset_response.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiConsumerResetResponse) Schema() ([]byte, error) {
	return scfs.Load("jetstream/api/v1/consumer_reset_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerUnpinRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_unpin_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiConsumerUnpinRequest) ApiSubjectPattern() (string, error) {
	return JSApiConsumerUnpin, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiConsumerUnpinRequest) ApiSubjectFormat() (string, error) {
	return JSApiConsumerUnpinT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiConsumerUnpinRequest) ApiSubjectPrefix() (string, error) {
	return JSApiConsumerUnpinPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiConsumerUnpinResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/consumer_unpin_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiLeaderStepDownRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/meta_leader_stepdown_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiLeaderStepDownRequest) ApiSubjectPattern() (string, error) {
	return JSApiLeaderStepDown, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiLeaderStepDownRequest) ApiSubjectFormat() (string, error) {
	return JSApiLeaderStepDownT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiLeaderStepDownRequest) ApiSubjectPrefix() (string, error) {
	return JSApiLeaderStepDownPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiLeaderStepDownResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/meta_leader_stepdown_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMetaServerRemoveRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/meta_server_remove_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiMetaServerRemoveRequest) ApiSubjectPattern() (string, error) {
	return JSApiServerRemove, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiMetaServerRemoveRequest) ApiSubjectFormat() (string, error) {
	return JSApiServerRemoveT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiMetaServerRemoveRequest) ApiSubjectPrefix() (string, error) {
	return JSApiServerRemovePrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMetaServerRemoveResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/meta_server_remove_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSPubAckResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/pub_ack_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t StreamConfig) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_configuration.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamCreateRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_create_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamCreateRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamCreate, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamCreateRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamCreateT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamCreateRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamCreatePrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamCreateResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_create_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamDeleteResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_delete_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamInfoRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_info_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamInfoRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamInfo, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamInfoRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamInfoT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamInfoRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamInfoPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamInfoResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_info_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamLeaderStepDownRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_leader_stepdown_request
func (t JSApiStreamLeaderStepDownRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_leader_stepdown_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamLeaderStepDownRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_leader_stepdown_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamLeaderStepDownRequest) Schema() ([]byte, error) {
	return scfs.Load("jetstream/api/v1/stream_leader_stepdown_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamLeaderStepDownRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamLeaderStepDown, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamLeaderStepDownRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamLeaderStepDownT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamLeaderStepDownRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamLeaderStepDownPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamLeaderStepDownResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_leader_stepdown_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamListRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_list_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamListRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamList, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamListRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamListT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamListRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamListPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamListResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_list_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMsgDeleteRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_msg_delete_request
func (t JSApiMsgDeleteRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_msg_delete_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiMsgDeleteRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_msg_delete_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiMsgDeleteRequest) Schema() ([]byte, error) {
	return scfs.Load("jetstream/api/v1/stream_msg_delete_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiMsgDeleteRequest) ApiSubjectPattern() (string, error) {
	return JSApiMsgDelete, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiMsgDeleteRequest) ApiSubjectFormat() (string, error) {
	return JSApiMsgDeleteT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiMsgDeleteRequest) ApiSubjectPrefix() (string, error) {
	return JSApiMsgDeletePrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMsgDeleteResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_msg_delete_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMsgGetRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_msg_get_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiMsgGetRequest) ApiSubjectPattern() (string, error) {
	return JSApiMsgGet, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiMsgGetRequest) ApiSubjectFormat() (string, error) {
	return JSApiMsgGetT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiMsgGetRequest) ApiSubjectPrefix() (string, error) {
	return JSApiMsgGetPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiMsgGetResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_msg_get_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamNamesRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_names_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamNamesRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamNames, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamNamesRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamNamesT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamNamesRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamNamesPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamNamesResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_names_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamPurgeRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_purge_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamPurgeRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamPurge, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamPurgeRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamPurgeT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamPurgeRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamPurgePrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamPurgeResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_purge_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamRemovePeerRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_remove_peer_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamRemovePeerRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamRemovePeer, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamRemovePeerRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamRemovePeerT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamRemovePeerRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamRemovePeerPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamRemovePeerResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_remove_peer_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamRestoreRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_restore_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamRestoreRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamRestore, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamRestoreRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamRestoreT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamRestoreRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamRestorePrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamRestoreResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_restore_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamSnapshotRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_snapshot_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamSnapshotRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamSnapshot, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamSnapshotRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamSnapshotT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamSnapshotRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamSnapshotPrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamSnapshotResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_snapshot_response.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamUpdateRequest) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.api.v1.stream_update_request
func (t JSApiStreamUpdateRequest) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_update_request"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSApiStreamUpdateRequest) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/api/v1/stream_update_request.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSApiStreamUpdateRequest) Schema() ([]byte, error) {
	return scfs.Load("jetstream/api/v1/stream_update_request.json")
}

// ApiSubjectPattern returns the NATS subject for the API request subject, may include NATS Subject wildcards
func (t JSApiStreamUpdateRequest) ApiSubjectPattern() (string, error) {
	return JSApiStreamUpdate, nil
}

// ApiSubjectFormat returns the NATS subject for the API request subject usable with Sprintf()
func (t JSApiStreamUpdateRequest) ApiSubjectFormat() (string, error) {
	return JSApiStreamUpdateT, nil
}

// ApiSubjectPrefix returns the NATS subject for the API request subject that prefixes any patterns or stream/consumer specific names
func (t JSApiStreamUpdateRequest) ApiSubjectPrefix() (string, error) {
	return JSApiStreamUpdatePrefix, nil
}

// Validate performs a JSON Schema validation of the configuration
func (t JSApiStreamUpdateResponse) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
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
	return scfs.Load("jetstream/api/v1/stream_update_response.json")
}
