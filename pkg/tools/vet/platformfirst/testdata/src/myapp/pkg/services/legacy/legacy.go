package legacy

type Platform interface{ DB() any }

type manager struct{}

func New(repo any, platform Platform) *manager { return &manager{} } // want `New does not take the platform first`
