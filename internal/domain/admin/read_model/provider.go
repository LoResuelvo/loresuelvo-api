package readmodel

import (
	"time"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
)

// Provider is the administrative projection of a registered provider.
type Provider struct {
	ID                         int
	Name                       string
	Surname                    string
	Email                      string
	ProfilePhotoFileID         string
	ProfilePhotoURL            string
	CreatedOn                  time.Time
	Category                   ProviderCategory
	CoverageZones              []ProviderCoverageZone
	IdentityVerificationStatus identityverification.VerificationStatus
	IdentityVerifiedOn         *time.Time
}

type ProviderCategory struct {
	ID   int
	Name string
}

type ProviderCoverageZone struct {
	ID           int
	MarketID     int
	Code         string
	Name         string
	Kind         coveragezone.Kind
	ParentZoneID *int
	Enabled      bool
}
