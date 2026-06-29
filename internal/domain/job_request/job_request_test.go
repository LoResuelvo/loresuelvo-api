package jobrequest_test

import (
	"testing"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
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

func TestNewJobRequestStartsPending(t *testing.T) {
	jobRequest, err := jobrequest.New(10, 20, "Reparación", "Necesito ayuda", []filedomain.MessageImage{})

	assert.NoError(t, err)
	assert.Equal(t, jobrequest.StatusPending, jobRequest.Status)
}

func TestNewJobRequestRejectsMoreThanThreeImages(t *testing.T) {
	jobRequest, err := jobrequest.New(10, 20, "Reparación", "Necesito ayuda", []filedomain.MessageImage{
		{FileID: "file-1"},
		{FileID: "file-2"},
		{FileID: "file-3"},
		{FileID: "file-4"},
	})

	assert.ErrorIs(t, err, jobrequest.ErrJobRequestImageNotAvailable)
	assert.Nil(t, jobRequest)
}

func TestAcceptPendingJobRequest(t *testing.T) {
	jobRequest := jobrequest.JobRequest{
		ProviderID: 10,
		Status:     jobrequest.StatusPending,
	}

	err := jobRequest.Accept(10)

	assert.NoError(t, err)
	assert.Equal(t, jobrequest.StatusAccepted, jobRequest.Status)
}

func TestRejectAcceptingAcceptedJobRequest(t *testing.T) {
	jobRequest := jobrequest.JobRequest{
		ProviderID: 10,
		Status:     jobrequest.StatusAccepted,
	}

	err := jobRequest.Accept(10)

	assert.ErrorIs(t, err, jobrequest.ErrOnlyPendingJobRequestCanBeAccepted)
}

func TestOpenStatuses(t *testing.T) {
	assert.Equal(t, []jobrequest.Status{jobrequest.StatusPending, jobrequest.StatusAccepted}, jobrequest.OpenStatuses())
}
