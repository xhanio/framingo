package order

type Platform interface{ DB() any }

type Manager interface{}

type manager struct{}

func New(platform Platform, repo any) Manager { return &manager{} }

// only the constructor named New is held to it
func NewForTest(repo any) Manager { return &manager{} }
