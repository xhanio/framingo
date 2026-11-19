package preset

// The system's own CAs, one per purpose: each is created on first Init and
// never listed nor deleted.
const (
	ServerCAName = "server-ca" // signs the manager's own certificates; agents verify the manager against it
	AgentCAName  = "agent-ca"  // signs the certificates agents present; the manager verifies agents against it
)
