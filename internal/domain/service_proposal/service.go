package serviceproposal

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Service struct {
	providerRepository ProviderRepository
	consumerRepository ConsumerRepository
}

func NewService(providerRepo ProviderRepository, consumerRepo ConsumerRepository) *Service {
	return &Service{
		providerRepository: providerRepo,
		consumerRepository: consumerRepo,
	}
}

func (s *Service) CreateServiceProposal(auth0ID string, consumerID int, amount int64, scheduledOn time.Time, description string) (*ServiceProposal, error) {
	provider, consumer, err := s.getParticipants(auth0ID, consumerID)
	if err != nil {
		return nil, err
	}

	return NewServiceProposal(provider, consumer, amount, scheduledOn, description)
}

func (s *Service) getParticipants(providerAuth0ID string, consumerID int) (*provider.Provider, *consumer.Consumer, error) {
	provider, err := s.providerRepository.FindByAuthID(providerAuth0ID)
	if err != nil {
		return nil, nil, ErrProviderRequired
	}

	consumer, err := s.consumerRepository.FindByID(consumerID)
	if err != nil {
		return nil, nil, ErrConsumerRequired
	}

	return provider, consumer, nil
}
