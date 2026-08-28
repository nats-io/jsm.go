// auto generated 2026-08-28 10:23:33.135778 +0200 CEST m=+0.837525376
package advisory

import (
	"github.com/nats-io/jsm.go/registry/validator"
	scfs "github.com/nats-io/jsm.go/schemas"
)

// Validate performs a JSON Schema validation of the configuration
func (t JetStreamAPIAuditV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.api_audit
func (t JetStreamAPIAuditV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.api_audit"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JetStreamAPIAuditV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/api_audit.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JetStreamAPIAuditV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/api_audit.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSConsumerActionAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.consumer_action
func (t JSConsumerActionAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.consumer_action"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSConsumerActionAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/consumer_action.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSConsumerActionAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/consumer_action.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSConsumerGroupPinnedAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.consumer_group_pinned
func (t JSConsumerGroupPinnedAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.consumer_group_pinned"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSConsumerGroupPinnedAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/consumer_group_pinned.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSConsumerGroupPinnedAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/consumer_group_pinned.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSConsumerGroupUnPinnedAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.consumer_group_unpinned
func (t JSConsumerGroupUnPinnedAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.consumer_group_unpinned"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSConsumerGroupUnPinnedAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/consumer_group_unpinned.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSConsumerGroupUnPinnedAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/consumer_group_unpinned.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSConsumerLeaderElectedV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.consumer_leader_elected
func (t JSConsumerLeaderElectedV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.consumer_leader_elected"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSConsumerLeaderElectedV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/consumer_leader_elected.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSConsumerLeaderElectedV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/consumer_leader_elected.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSConsumerPauseAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.consumer_pause
func (t JSConsumerPauseAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.consumer_pause"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSConsumerPauseAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/consumer_pause.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSConsumerPauseAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/consumer_pause.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSConsumerQuorumLostV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.consumer_quorum_lost
func (t JSConsumerQuorumLostV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.consumer_quorum_lost"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSConsumerQuorumLostV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/consumer_quorum_lost.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSConsumerQuorumLostV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/consumer_quorum_lost.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSDomainLeaderElectedV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.domain_leader_elected
func (t JSDomainLeaderElectedV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.domain_leader_elected"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSDomainLeaderElectedV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/domain_leader_elected.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSDomainLeaderElectedV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/domain_leader_elected.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t ConsumerDeliveryExceededAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.max_deliver
func (t ConsumerDeliveryExceededAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.max_deliver"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t ConsumerDeliveryExceededAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/max_deliver.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t ConsumerDeliveryExceededAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/max_deliver.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSConsumerDeliveryNakAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.nak
func (t JSConsumerDeliveryNakAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.nak"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSConsumerDeliveryNakAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/nak.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSConsumerDeliveryNakAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/nak.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSRestoreCompleteAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.restore_complete
func (t JSRestoreCompleteAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.restore_complete"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSRestoreCompleteAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/restore_complete.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSRestoreCompleteAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/restore_complete.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSRestoreCreateAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.restore_create
func (t JSRestoreCreateAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.restore_create"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSRestoreCreateAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/restore_create.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSRestoreCreateAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/restore_create.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSServerOutOfSpaceAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.server_out_of_space
func (t JSServerOutOfSpaceAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.server_out_of_space"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSServerOutOfSpaceAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/server_out_of_space.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSServerOutOfSpaceAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/server_out_of_space.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSServerRemovedAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.server_removed
func (t JSServerRemovedAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.server_removed"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSServerRemovedAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/server_removed.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSServerRemovedAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/server_removed.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSSnapshotCompleteAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.snapshot_complete
func (t JSSnapshotCompleteAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.snapshot_complete"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSSnapshotCompleteAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/snapshot_complete.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSSnapshotCompleteAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/snapshot_complete.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSSnapshotCreateAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.snapshot_create
func (t JSSnapshotCreateAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.snapshot_create"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSSnapshotCreateAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/snapshot_create.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSSnapshotCreateAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/snapshot_create.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSStreamActionAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.stream_action
func (t JSStreamActionAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.stream_action"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSStreamActionAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/stream_action.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSStreamActionAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/stream_action.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSStreamBatchAbandonedAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.stream_batch_abandoned
func (t JSStreamBatchAbandonedAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.stream_batch_abandoned"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSStreamBatchAbandonedAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/stream_batch_abandoned.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSStreamBatchAbandonedAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/stream_batch_abandoned.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSStreamLeaderElectedV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.stream_leader_elected
func (t JSStreamLeaderElectedV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.stream_leader_elected"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSStreamLeaderElectedV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/stream_leader_elected.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSStreamLeaderElectedV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/stream_leader_elected.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSStreamQuorumLostV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.stream_quorum_lost
func (t JSStreamQuorumLostV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.stream_quorum_lost"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSStreamQuorumLostV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/stream_quorum_lost.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSStreamQuorumLostV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/stream_quorum_lost.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t JSConsumerDeliveryTerminatedAdvisoryV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.advisory.v1.terminated
func (t JSConsumerDeliveryTerminatedAdvisoryV1) SchemaType() string {
	return "io.nats.jetstream.advisory.v1.terminated"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t JSConsumerDeliveryTerminatedAdvisoryV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/advisory/v1/terminated.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t JSConsumerDeliveryTerminatedAdvisoryV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/advisory/v1/terminated.json")
}
