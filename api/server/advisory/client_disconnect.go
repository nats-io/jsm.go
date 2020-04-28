package advisory

// DisconnectEventMsgV1 is sent when a new connection previously defined from a
// ConnectEventMsg is closed.
//
// NATS Schema Type io.nats.server.advisory.v1.client_disconnect
type DisconnectEventMsgV1 struct {
	Type     string       `json:"type"`
	ID       string       `json:"id"`
	Time     string       `json:"timestamp"`
	Server   ServerInfoV1 `json:"server"`
	Client   ClientInfoV1 `json:"client"`
	Sent     DataStatsV1  `json:"sent"`
	Received DataStatsV1  `json:"received"`
	Reason   string       `json:"reason"`
}
