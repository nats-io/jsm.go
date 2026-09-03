// auto generated 2026-08-28 10:23:32.979313 +0200 CEST m=+0.681063209

package registry

import (
	"github.com/nats-io/jsm.go/api/jetstream/advisory"
)

func init() {
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.api_audit", func() any { return &advisory.JetStreamAPIAuditV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.consumer_action", func() any { return &advisory.JSConsumerActionAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.consumer_group_pinned", func() any { return &advisory.JSConsumerGroupPinnedAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.consumer_group_unpinned", func() any { return &advisory.JSConsumerGroupUnPinnedAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.consumer_leader_elected", func() any { return &advisory.JSConsumerLeaderElectedV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.consumer_pause", func() any { return &advisory.JSConsumerPauseAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.consumer_quorum_lost", func() any { return &advisory.JSConsumerQuorumLostV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.domain_leader_elected", func() any { return &advisory.JSDomainLeaderElectedV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.max_deliver", func() any { return &advisory.ConsumerDeliveryExceededAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.nak", func() any { return &advisory.JSConsumerDeliveryNakAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.restore_complete", func() any { return &advisory.JSRestoreCompleteAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.restore_create", func() any { return &advisory.JSRestoreCreateAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.server_out_of_space", func() any { return &advisory.JSServerOutOfSpaceAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.server_removed", func() any { return &advisory.JSServerRemovedAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.snapshot_complete", func() any { return &advisory.JSSnapshotCompleteAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.snapshot_create", func() any { return &advisory.JSSnapshotCreateAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.stream_action", func() any { return &advisory.JSStreamActionAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.stream_batch_abandoned", func() any { return &advisory.JSStreamBatchAbandonedAdvisoryV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.stream_leader_elected", func() any { return &advisory.JSStreamLeaderElectedV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.stream_quorum_lost", func() any { return &advisory.JSStreamQuorumLostV1{} })
	RegisterTypeFactory("io.nats.jetstream.advisory.v1.terminated", func() any { return &advisory.JSConsumerDeliveryTerminatedAdvisoryV1{} })
}
