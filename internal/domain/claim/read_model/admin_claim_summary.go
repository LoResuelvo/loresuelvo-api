package readmodel

// AdminClaimSummary extends the existing projection without loading testimony or evidence.
type AdminClaimSummary struct {
	ClaimSummary
	ClaimantID    int
	ClaimantParty string
	ClaimantName  string
	ClaimantEmail string
	CategoryID    *int
	CategoryName  *string
	AgeSeconds    int64
}

// AdminIdentity is a participant projection for administrative claim details.
type AdminIdentity struct {
	ID    int
	Party string
	Name  string
	Email string
}
