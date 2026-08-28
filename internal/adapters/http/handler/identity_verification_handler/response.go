package identity_verification_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
)

type sessionResponse struct {
	SessionID       uuid.UUID                               `json:"session_id"`
	SessionToken    string                                  `json:"session_token"`
	VerificationURL string                                  `json:"verification_url"`
	Status          identityverification.VerificationStatus `json:"status"`
}
