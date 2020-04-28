package metric

// ConsumerAckMetricV1 is a metric published when a Consumer
// has ACK sampling enabled to indicate message processing stats
//
// NATS Schema Type io.nats.jetstream.metric.v1.consumer_ack
type ConsumerAckMetricV1 struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Time        string `json:"timestamp"`
	Stream      string `json:"stream"`
	Consumer    string `json:"consumer"`
	ConsumerSeq uint64 `json:"consumer_seq"`
	StreamSeq   uint64 `json:"stream_seq"`
	Delay       int64  `json:"ack_time"`
	Deliveries  uint64 `json:"deliveries"`
}
