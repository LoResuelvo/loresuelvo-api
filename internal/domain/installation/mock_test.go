package installation

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

func validRegistration() Registration {
	return Registration{ID: uuid.NewString(), Secret: uuid.NewString(), App: "consumer", Token: "registered-token", BindingID: uuid.NewString()}
}

type repositoryMock struct{ mock.Mock }

func (m *repositoryMock) FindByID(ctx context.Context, id string) (*Installation, error) {
	a := m.Called(ctx, id)
	var i *Installation
	if a.Get(0) != nil {
		i = a.Get(0).(*Installation)
	}
	return i, a.Error(1)
}
func (m *repositoryMock) Save(ctx context.Context, i *Installation) error {
	return m.Called(ctx, i).Error(0)
}

type userFinderMock struct{ mock.Mock }

func (m *userFinderMock) FindByAuthID(authID string) (user.User, error) {
	a := m.Called(authID)
	var u user.User
	if a.Get(0) != nil {
		u = a.Get(0).(user.User)
	}
	return u, a.Error(1)
}
