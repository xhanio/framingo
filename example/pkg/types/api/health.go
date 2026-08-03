package api

// ServiceReadiness names one supervised service that is not ready, and why.
type ServiceReadiness struct {
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
}

// ReadyzResponse is the /readyz body: ready with no services listed, or not
// ready with the offenders.
type ReadyzResponse struct {
	Ready    bool               `json:"ready"`
	Services []ServiceReadiness `json:"services,omitempty"`
}
