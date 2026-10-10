package push

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobRequestTemplatesDoNotPreviewPrivateContent(t *testing.T) {
	require.Equal(t, [2][2]string{
		{"Nueva solicitud de trabajo", "Recibiste una solicitud de trabajo."},
		{"New job request", "You received a job request."},
	}, templates["job_request_received"])
}
