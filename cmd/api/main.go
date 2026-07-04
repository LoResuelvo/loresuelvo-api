package main

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/bootstrap"

	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"

	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
)

func main() {
	database, err := db.ConnectPostgres(context.Background(), db.NewPostgresConfigFromEnv())
	if err != nil {
		panic(err)
	}
	defer database.Close()

	bootstrap.StartDefaultDataSeederFromEnv(context.Background(), database)

	dependencies := bootstrap.NewDependencies(database)

	auth0Validator, err := auth0.NewValidatorFromEnv()
	if err != nil {
		panic(err)
	}

	router := httpadapter.NewRouter(dependencies.CategoryHandler, dependencies.ConsumerHandler, dependencies.ProviderHandler, dependencies.ConversationHandler, dependencies.JobRequestHandler, dependencies.UserHandler, dependencies.FileHandler, dependencies.TestHandler, dependencies.RealtimeHandler, auth0Validator)
	engine, err := router.SetUp()
	if err != nil {
		panic(err)
	}

	if err := engine.Run(":8080"); err != nil {
		panic(err)
	}
}
