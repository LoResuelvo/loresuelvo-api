package main

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"

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
	consumerManager := consumer.NewService(consumerRepository)
	consumerHandler := handler.NewConsumerHandler(consumerManager)

	authMiddleware, err := httpadapter.NewAuth0MiddlewareFromEnv()
	if err != nil {
		panic(err)
	}

	router := httpadapter.NewRouter(consumerHandler, authMiddleware)
	engine := router.SetUp()

	if err := engine.Run(":8080"); err != nil {
		panic(err)
	}
}
