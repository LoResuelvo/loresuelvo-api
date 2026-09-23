package admin

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"

// ProviderDirectoryFilter selects providers without changing the data returned for each match.
type ProviderDirectoryFilter struct {
	Query                      string
	CategoryID                 *int
	CoverageZoneID             *int
	IdentityVerificationStatus *identityverification.VerificationStatus
}
