package main

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/bootstrap"

	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"

	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.ConnectPostgres(ctx, db.NewPostgresConfigFromEnv())
	if err != nil {
		panic(err)
	}
	defer database.Close()

	bootstrap.StartDefaultDataSeederFromEnv(ctx, database)

	dependencies, err := bootstrap.NewDependencies(database)
	if err != nil {
		panic(err)
	}
	go dependencies.UrgentWorkOrderScheduler.Run(ctx)

	auth0Validator, err := auth0.NewValidatorFromEnv()
	if err != nil {
		panic(err)
	}

	router := httpadapter.NewRouter(dependencies.RouterConfig(auth0Validator))
	engine, err := router.SetUp()
	if err != nil {
		panic(err)
	}

	if err := engine.Run(":8080"); err != nil {
		panic(err)
	}
}
