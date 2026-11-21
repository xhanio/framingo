package a

import (
	"context"
	"time"
)

type keeps struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (m *keeps) Start(ctx context.Context) error {
	m.ctx = ctx // want `Start keeps its ctx in m.ctx`
	return nil
}

type derives struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (m *derives) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx) // want `Start keeps the cancel of a context derived from its ctx in m.cancel`
	m.ctx = ctx                             // want `Start keeps its ctx in m.ctx`
	return nil
}

type viaLocal struct{ cancel context.CancelFunc }

func (m *viaLocal) Start(ctx context.Context) error {
	run, cancel := context.WithCancel(ctx)
	m.cancel = cancel // want `Start keeps its ctx in m.cancel`
	go func() {
		<-run.Done() // want `Start hands its ctx to a goroutine it leaves running`
	}()
	return nil
}

type goes struct{}

func (m *goes) Start(ctx context.Context) error {
	go m.loop(ctx) // want `Start hands its ctx to a goroutine it leaves running`
	return nil
}

func (m *goes) loop(context.Context) {}

// what a start runs keeps the values, never the cancel
type cuts struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (m *cuts) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(context.WithoutCancel(ctx))
	run := context.WithoutCancel(ctx)
	go func() { <-run.Done() }()
	return nil
}

// the parameter's name reused for the context that outlives the call
type reuses struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (m *reuses) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(context.WithoutCancel(ctx))
	m.ctx = ctx
	go func() { <-ctx.Done() }()
	return nil
}

// a directive with its reason opts a line out; one without does not
type excused struct {
	ctx   context.Context
	other context.Context
}

func (m *excused) Start(ctx context.Context) error {
	//framingo:ignore startctx the caller's ctx is this type's whole purpose
	m.ctx = ctx
	//framingo:ignore startctx
	m.other = ctx // want `Start keeps its ctx in m.other`
	return nil
}

// bounding the start itself is what the ctx is for
type bounds struct{}

func (m *bounds) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return m.dial(ctx)
}

func (m *bounds) dial(context.Context) error { return nil }

// not a Daemon's Start: other names and other signatures are left alone
type other struct{ ctx context.Context }

func (m *other) Init(ctx context.Context) error { m.ctx = ctx; return nil }
func (m *other) Start(ctx context.Context, n int) error {
	m.ctx = ctx
	return nil
}
