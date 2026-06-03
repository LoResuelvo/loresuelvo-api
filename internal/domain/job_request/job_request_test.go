package jobrequest_test

import (
	"testing"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/stretchr/testify/assert"
)

func TestCheckIfAJobRequestCanBeActivated(t *testing.T) {
	jobRequest := jobrequest.JobRequest{
		ProviderID: 10,
	}

	assert.True(t, jobRequest.CanBeAcceptedBy(10))
	assert.False(t, jobRequest.CanBeAcceptedBy(20))
}
