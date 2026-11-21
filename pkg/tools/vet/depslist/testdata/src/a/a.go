package a

type Service interface {
	Name() string
	Dependencies() []Service
}

type Sender interface {
	Service
	Send(string)
}

type impl struct{}

func (*impl) Name() string            { return "impl" }
func (*impl) Dependencies() []Service { return nil }

type forgets struct {
	repo  Service
	bus   Sender // want `field bus holds a service that Dependencies does not list`
	named string
}

func (m *forgets) Name() string { return "forgets" }
func (m *forgets) Dependencies() []Service {
	return []Service{m.repo}
}

type concrete struct {
	held    *impl // want `field held holds a service that Dependencies does not list`
	byValue impl  // want `field byValue holds a service that Dependencies does not list`
}

func (m *concrete) Name() string            { return "concrete" }
func (m *concrete) Dependencies() []Service { return nil }

// every held service is listed, however the list is built
type lists struct {
	repo     Service
	bus      Sender
	optional Service
	peers    []Service
	count    int
}

func (m *lists) Name() string { return "lists" }
func (m *lists) Dependencies() []Service {
	deps := []Service{m.repo, m.bus}
	deps = append(deps, m.optional)
	return append(deps, m.peers...)
}

// a directive with its reason opts a field out
type excused struct {
	//framingo:ignore depslist telemetry never gates a service
	meter Service
	bus   Sender // want `field bus holds a service that Dependencies does not list`
}

func (m *excused) Name() string            { return "excused" }
func (m *excused) Dependencies() []Service { return nil }

// a filter in Dependencies: the supervisor skips a nil entry itself
type filters struct {
	repo     Service
	optional Service
	bus      Service
}

func (m *filters) Name() string { return "filters" }
func (m *filters) Dependencies() []Service {
	deps := []Service{m.repo}
	if m.optional != nil { // want `Dependencies filters what it lists`
		deps = append(deps, m.optional)
	}
	if _, ok := m.bus.(Sender); ok { // want `Dependencies filters what it lists` `Dependencies filters what it lists`
		deps = append(deps, m.bus)
	}
	return deps
}

// a directive with its reason keeps a condition the list needs
type guarded struct{ repo Service }

func (m *guarded) Name() string { return "guarded" }
func (m *guarded) Dependencies() []Service {
	//framingo:ignore depslist a reason the test gives
	if m.repo == nil {
		return nil
	}
	return []Service{m.repo}
}

// not a service's Dependencies: another signature is left alone
type other struct{ repo Service }

func (m *other) Dependencies(deep bool) []Service { return nil }
