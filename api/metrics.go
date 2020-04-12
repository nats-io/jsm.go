package api

type ConsumerAckMetric struct {
	Schema      string `json:"schema"`
	ID          string `json:"id"`
	Time        string `json:"timestamp"`
	Stream      string `json:"stream"`
	Consumer    string `json:"consumer"`
	ConsumerSeq uint64 `json:"consumer_seq"`
	StreamSeq   uint64 `json:"stream_seq"`
	Delay       int64  `json:"ack_time"`
	Deliveries  uint64 `json:"deliveries"`
}

type ConsumerDeliveryExceededAdvisory struct {
	Schema     string `json:"schema"`
	ID         string `json:"id"`
	Time       string `json:"timestamp"`
	Stream     string `json:"stream"`
	Consumer   string `json:"consumer"`
	StreamSeq  uint64 `json:"stream_seq"`
	Deliveries uint64 `json:"deliveries"`
}

type JetStreamAPIAudit struct {
	Schema   string         `json:"schema"`
	ID       string         `json:"id"`
	Time     string         `json:"timestamp"`
	Server   string         `json:"server"`
	Client   ClientAPIAudit `json:"client"`
	Subject  string         `json:"subject"`
	Request  string         `json:"request,omitempty"`
	Response string         `json:"response"`
}

type ClientAPIAudit struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	CID      uint64 `json:"cid"`
	Account  string `json:"account"`
	User     string `json:"user,omitempty"`
	Name     string `json:"name,omitempty"`
	Language string `json:"lang,omitempty"`
	Version  string `json:"version,omitempty"`
}
