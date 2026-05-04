package main

import (
	"github.com/LoResuelvo/loresuelvo-api/pkg/router"
)

func main() {
	r := router.SetupRouter()
	err := r.Run(":8080")
	if err != nil {
		panic(err)
	}
}
