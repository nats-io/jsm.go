package advisory

// ConsumerDeliveryExceededAdvisoryV1 is an advisory published when a consumer
// message reaches max delivery attempts
//
// NATS Schema Type io.nats.jetstream.advisory.v1.max_deliver
type ConsumerDeliveryExceededAdvisoryV1 struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Time       string `json:"timestamp"`
	Stream     string `json:"stream"`
	Consumer   string `json:"consumer"`
	StreamSeq  uint64 `json:"stream_seq"`
	Deliveries uint64 `json:"deliveries"`
}
