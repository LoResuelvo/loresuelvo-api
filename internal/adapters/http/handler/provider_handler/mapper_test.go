package provider_handler

import (
	"encoding/json"
	"testing"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/require"
)

func TestProviderProfileResponseExposesOnlyPublicIdentityApproval(t *testing.T) {
	response := providerProfileResponseFromReadModel(readmodel.Profile{
		ID:               12,
		IdentityVerified: true,
	})
	payload, err := json.Marshal(response)
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &fields))
	require.Contains(t, fields, "identity_verified")
	for _, forbiddenField := range []string{
		"identity_verification_status",
		"identity_verified_on",
		"risk_codes",
		"session_id",
		"workflow_id",
		"email",
		"auth_id",
		"identity_data",
		"document_number",
	} {
		require.NotContains(t, fields, forbiddenField)
	}
}
