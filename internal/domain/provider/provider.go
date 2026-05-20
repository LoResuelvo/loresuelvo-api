package provider

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/user"

type Provider struct {
	User                   *user.User
	CoverageZones          []string
	CriminalRecord         string
	ProfessionalCredential string
}

func NewProvider(auth0ID string, email string, name string, surname string, coverageZone []string) (*Provider, error) {
	user, err := user.New(auth0ID, name, surname, email, "provider")
	if err != nil {
		return nil, err
	}

	return &Provider{
		User:          user,
		CoverageZones: coverageZone,
	}, nil
}
