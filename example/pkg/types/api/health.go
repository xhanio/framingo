package api

type ServiceReadiness struct {
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
}

type ReadyzResponse struct {
	Ready    bool               `json:"ready"`
	Services []ServiceReadiness `json:"services,omitempty"`
}
