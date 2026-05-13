package main

import (
	"context"

	usecases "github.com/LoResuelvo/loresuelvo-api/app/use_cases"
	"github.com/LoResuelvo/loresuelvo-api/db"
	"github.com/LoResuelvo/loresuelvo-api/persistence"
	"github.com/LoResuelvo/loresuelvo-api/pkg/router"
)

func main() {
	database, err := db.ConnectPostgres(context.Background(), db.NewPostgresConfigFromEnv())
	if err != nil {
		panic(err)
	}
	defer database.Close()

	consumerRepository := persistence.NewConsumerRepository(database)
	consumerManager := usecases.NewConsumerManager(consumerRepository)

	r := router.NewRouter()
	router.RegisterConsumerRoutes(r, consumerManager)

	err = r.Run(":8080")
	if err != nil {
		panic(err)
	}
}
