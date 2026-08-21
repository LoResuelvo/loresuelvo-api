package coveragezone

import "strings"

type Kind string

const (
	KindCommune         Kind = "COMMUNE"
	KindParty           Kind = "PARTY"
	KindDepartment      Kind = "DEPARTMENT"
	KindNeighborhood    Kind = "NEIGHBORHOOD"
	KindOperationalZone Kind = "OPERATIONAL_ZONE"
)

type Market struct {
	ID      int
	Code    string
	Name    string
	Enabled bool
}

type ExternalReference struct {
	CoverageZoneID int
	Provider       string
	ExternalID     string
	SourceVersion  string
}

func NewExternalReference(coverageZoneID int, provider, externalID, sourceVersion string) (*ExternalReference, error) {
	if coverageZoneID <= 0 {
		return nil, ErrCoverageZoneRequired
	}
	normalizedProvider := strings.ToUpper(strings.TrimSpace(provider))
	if normalizedProvider == "" {
		return nil, ErrExternalProviderRequired
	}
	trimmedExternalID := strings.TrimSpace(externalID)
	if trimmedExternalID == "" {
		return nil, ErrExternalIDRequired
	}

	return &ExternalReference{
		CoverageZoneID: coverageZoneID,
		Provider:       normalizedProvider,
		ExternalID:     trimmedExternalID,
		SourceVersion:  strings.TrimSpace(sourceVersion),
	}, nil
}

func NewMarket(code, name string) (*Market, error) {
	normalizedCode := strings.ToUpper(strings.TrimSpace(code))
	if normalizedCode == "" {
		return nil, ErrCodeRequired
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrNameRequired
	}

	return &Market{Code: normalizedCode, Name: trimmedName, Enabled: true}, nil
}

type CoverageZone struct {
	ID             int
	MarketID       int
	Code           string
	Name           string
	NormalizedName string
	Kind           Kind
	ParentZoneID   *int
	Enabled        bool
}

func New(marketID int, code, name string, kind Kind) (*CoverageZone, error) {
	if marketID <= 0 {
		return nil, ErrMarketRequired
	}
	normalizedCode := strings.ToUpper(strings.TrimSpace(code))
	if normalizedCode == "" {
		return nil, ErrCodeRequired
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrNameRequired
	}
	if kind == "" {
		return nil, ErrKindRequired
	}
	if !kind.IsValid() {
		return nil, ErrKindInvalid
	}

	return &CoverageZone{
		MarketID:       marketID,
		Code:           normalizedCode,
		Name:           trimmedName,
		NormalizedName: strings.ToLower(trimmedName),
		Kind:           kind,
		Enabled:        true,
	}, nil
}

func (kind Kind) IsValid() bool {
	switch kind {
	case KindCommune, KindParty, KindDepartment, KindNeighborhood, KindOperationalZone:
		return true
	default:
		return false
	}
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

func ValidateUniqueIDs(ids []int) error {
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			return ErrDuplicateCoverageZone
		}
		seen[id] = struct{}{}
	}

	return nil
}
