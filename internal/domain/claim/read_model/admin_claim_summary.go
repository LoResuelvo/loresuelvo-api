package readmodel

// AdminClaimSummary extends the existing projection without loading testimony or evidence.
type AdminClaimSummary struct {
	ClaimSummary
	ClaimantID    int
	ClaimantParty string
	ClaimantEmail string
	CategoryID    *int
	CategoryName  *string
	AgeSeconds    int64
}
