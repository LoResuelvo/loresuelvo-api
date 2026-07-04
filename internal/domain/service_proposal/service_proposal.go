package serviceproposal

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type ServiceProposal struct {
	provider    *provider.Provider
	consumer    *consumer.Consumer
	amount      int64
	scheduledOn time.Time
	description string
}

func NewServiceProposal(provider *provider.Provider, consumer *consumer.Consumer, amount int64, scheduledOn time.Time, description string) (*ServiceProposal, error) {
	return &ServiceProposal{
		provider:    provider,
		consumer:    consumer,
		amount:      amount,
		scheduledOn: scheduledOn,
		description: description,
	}, nil
}
