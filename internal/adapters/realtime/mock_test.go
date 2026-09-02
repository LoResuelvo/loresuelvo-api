package realtime

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/mock"
)

type eventBusMock struct {
	mock.Mock
}

func (bus *eventBusMock) Publish(ctx context.Context, event EventEnvelope) error {
	return bus.Called(ctx, event).Error(0)
}

func (bus *eventBusMock) Listen(ctx context.Context, handler func(EventEnvelope)) error {
	return bus.Called(ctx, handler).Error(0)
}

type notificationRecipientFinderStub struct {
	authID string
	role   string
	err    error
}

func (finder notificationRecipientFinderStub) FindByID(_ context.Context, id int) (user.User, error) {
	if finder.err != nil {
		return nil, finder.err
	}
	base := user.RehydrateBaseUser(id, finder.authID, "", "", "", finder.role, nil)
	if finder.role == provider.Role {
		return &provider.Provider{BaseUser: base}, nil
	}
	return &consumer.Consumer{BaseUser: base}, nil
}
