package example

import (
	"context"
)

type Client interface {
	Init() error
	HelloWorld(ctx context.Context, message string) error
	Login(ctx context.Context, username, password string) error
	Logout(ctx context.Context) error
	StreamMessages(ctx context.Context) error
}
