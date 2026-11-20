package example

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/traceutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

func (m *manager) HelloWorld(ctx context.Context, message string) (*entity.HelloWorld, error) {
	// child of the middleware's request span; every return Fails or Ends it
	ctx, span := traceutil.Start(ctx, m.name, "HelloWorld")

	credential, ok := entity.CredentialFrom(ctx)
	if !ok {
		return nil, traceutil.Fail(span, errors.Unauthorized.Newf("credential not found in context"))
	}
	m.mu.RLock()
	greeting := m.greeting
	m.mu.RUnlock()
	if greeting == "" {
		greeting = "hello world!"
	}
	m.log.Infof("%s %s from %s", greeting, message, credential.UserName)

	result, err := m.repository.CreateHelloWorld(ctx, message)
	if err != nil {
		return nil, traceutil.Fail(span, errors.Wrap(err))
	}
	m.log.Infof("saved helloworld message %s to database with ID: %d", message, result.ID)

	m.mb.SendRawMessage(ctx, m, "helloworld", result)
	m.inst.hello.Add(ctx, 1)
	span.End()
	return result, nil
}
