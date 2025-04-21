package api

type CreateServerRequest struct {
	JetStream bool `json:"jetstream"`
}

type ManagedServer struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster"`
	Port    int    `json:"port"`
	URL     string `json:"url,omitempty"`
	Running bool   `json:"running"`
}

type CreateResponse struct {
	Servers []*ManagedServer `json:"servers"`
}

type CreateClusterRequest struct {
	Servers   int  `json:"servers"`
	JetStream bool `json:"jetstream"`
}

type CreateSuperClusterRequest struct {
	Servers   int  `json:"servers"`
	Clusters  int  `json:"clusters"`
	JetStream bool `json:"jetstream" yaml:"jetstream"`
}

type ResetResponse struct {
	Shutdown bool `json:"shutdown"`
}

type StartServerRequest struct {
	Name string `json:"name"`
}

type StopServerRequest struct {
	Name string `json:"name"`
}

type StopServerResponse struct {
	Shutdown bool `json:"shutdown"`
}

type StartServerResponse struct {
	Started bool `json:"shutdown"`
}

type StatusResponse struct {
	Servers []ManagedServer `json:"servers"`
}
