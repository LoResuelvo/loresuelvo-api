package coveragezone

import "strings"

type CoverageZone struct {
	ID             int
	Name           string
	NormalizedName string
	Enabled        bool
}

func New(name string) (*CoverageZone, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrNameRequired
	}

	return &CoverageZone{
		Name:           trimmedName,
		NormalizedName: strings.ToLower(trimmedName),
		Enabled:        true,
	}, nil
}

func (zone *CoverageZone) Disable() {
	zone.Enabled = false
}

func (zone CoverageZone) ValidateSelection() error {
	if !zone.Enabled {
		return ErrNotAvailable
	}

	return nil
}
