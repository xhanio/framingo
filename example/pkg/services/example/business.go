package example

import (
	"context"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

func (m *manager) HelloWorld(ctx context.Context, message string) (*entity.HelloWorld, error) {
	credential, ok := ctx.Value(api.ContextKeyCredential).(*entity.Credential)
	if !ok {
		return nil, errors.Unauthorized.Newf("credential not found in context")
	}
	greeting := m.greeting
	if greeting == "" {
		greeting = "hello world!"
	}
	m.log.Infof("%s %s from %s", greeting, message, credential.UserName)

	ormModel, err := m.repository.CreateHelloWorld(ctx, message)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	m.log.Infof("saved helloworld message %s to database with ID: %d", message, ormModel.ID)

	result := &entity.HelloWorld{
		ID:        ormModel.ID,
		Message:   ormModel.Message,
		CreatedAt: ormModel.CreatedAt,
		UpdatedAt: ormModel.UpdatedAt,
	}
	m.mb.SendRawMessage(ctx, m, "helloworld", result)
	return result, nil
}
