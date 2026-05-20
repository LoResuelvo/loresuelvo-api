package bootstrap

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type Dependencies struct {
	UserRepository     *repositories.UserRepository
	ConsumerRepository *repositories.ConsumerRepository
	ProviderRepository *repositories.ProviderRepository

	ConsumerHandler *handler.ConsumerHandler
	ProviderHandler *handler.ProviderHandler
	UserHandler     *handler.UserHandler
}

func NewDependencies(database *sql.DB) *Dependencies {
	userRepository := repositories.NewUserRepository(database)
	consumerRepository := repositories.NewConsumerRepository(database, userRepository)
	providerRepository := repositories.NewProviderRepository(database, userRepository)

	providerService := provider.NewService(providerRepository)
	consumerService := consumer.NewService(consumerRepository)
	userService := user.NewService(userRepository)

	return &Dependencies{
		UserRepository:     userRepository,
		ConsumerRepository: consumerRepository,
		ProviderRepository: providerRepository,
		ConsumerHandler:    handler.NewConsumerHandler(consumerService),
		ProviderHandler:    handler.NewProviderHandler(providerService),
		UserHandler:        handler.NewUserHandler(userService),
	}
}
