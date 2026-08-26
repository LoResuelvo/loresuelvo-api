package repositories_test

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

// legacyConsumer models a persisted consumer created before addresses became mandatory.
func legacyConsumer(authID, email, name, surname string) *consumer.Consumer {
	baseUser, err := user.New(authID, name, surname, email, consumer.Role, nil)
	if err != nil {
		panic(err)
	}

	return consumer.RehydrateConsumer(baseUser, nil, nil, 0)
}
