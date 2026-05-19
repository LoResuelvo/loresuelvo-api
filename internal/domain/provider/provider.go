package provider

type Provider struct {
	Auth0ID                string
	Email                  string
	Name                   string
	Surname                string
	CoverageZones          []string
	CriminalRecord         string
	ProfessionalCredential string
}

func NewProvider(auth0ID string, email string, name string, surname string, coverageZone []string) Provider {
	return Provider{
		Auth0ID:       auth0ID,
		Email:         email,
		Name:          name,
		Surname:       surname,
		CoverageZones: coverageZone,
	}
}
