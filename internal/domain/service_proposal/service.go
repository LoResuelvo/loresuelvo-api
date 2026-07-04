package serviceproposal

import "time"

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) CreateServiceProposal(auth0ID string, consumerID int, amount int64, scheduledOn time.Time, description string) error {
	return nil
}
