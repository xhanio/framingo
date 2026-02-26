package common

type Named interface {
	Name() string
}

type Unique interface {
	Key() string
}

type Weighted interface {
	GetPriority() int
	SetPriority(priority int)
}
