package admin_handler

import "time"

type consumerDirectoryResponse struct {
	ID              int       `json:"id"`
	Role            string    `json:"role"`
	Name            string    `json:"name"`
	Surname         string    `json:"surname"`
	Email           string    `json:"email"`
	ProfilePhotoURL *string   `json:"profile_photo_url"`
	CreatedOn       time.Time `json:"created_on"`
}

type providerDirectoryResponse struct {
	ID                         int                             `json:"id"`
	Role                       string                          `json:"role"`
	Name                       string                          `json:"name"`
	Surname                    string                          `json:"surname"`
	Email                      string                          `json:"email"`
	ProfilePhotoURL            *string                         `json:"profile_photo_url"`
	CreatedOn                  time.Time                       `json:"created_on"`
	Category                   providerDirectoryCategory       `json:"category"`
	CoverageZones              []providerDirectoryCoverageZone `json:"coverage_zones"`
	IdentityVerificationStatus string                          `json:"identity_verification_status"`
	IdentityVerifiedOn         *time.Time                      `json:"identity_verified_on"`
}

type providerDirectoryCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type providerDirectoryCoverageZone struct {
	ID           int    `json:"id"`
	MarketID     int    `json:"market_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	ParentZoneID *int   `json:"parent_zone_id"`
	Enabled      bool   `json:"enabled"`
}
