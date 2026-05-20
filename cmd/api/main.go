package main

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"

	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"

	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
)

func main() {
	database, err := db.ConnectPostgres(context.Background(), db.NewPostgresConfigFromEnv())
	if err != nil {
		panic(err)
	}
	defer database.Close()

	consumerRepository := repositories.NewConsumerRepository(database)
	providerRepository := repositories.NewProviderRepository(database)

	consumerManager := consumer.NewService(consumerRepository)
	providerManager := provider.NewService(providerRepository)
	consumerHandler := handler.NewConsumerHandler(consumerManager)
	providerHandler := handler.NewProviderHandler(providerManager)

	auth0Validator, err := auth0.NewValidatorFromEnv()
	if err != nil {
		panic(err)
	}

	router := httpadapter.NewRouter(consumerHandler, providerHandler, auth0Validator)
	engine, err := router.SetUp()
	if err != nil {
		panic(err)
	}

	if err := engine.Run(":8080"); err != nil {
		panic(err)
	}
}
