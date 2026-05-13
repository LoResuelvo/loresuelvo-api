package main

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/db"
	"github.com/LoResuelvo/loresuelvo-api/pkg/router"
)

func main() {
	database, err := db.ConnectPostgres(context.Background(), db.NewPostgresConfigFromEnv())
	if err != nil {
		panic(err)
	}
	defer database.Close()

	r := router.SetupRouter(database)
	err = r.Run(":8080")
	if err != nil {
		panic(err)
	}
}
