package admin_test

import (
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestDiagnosticServiceAuditsOnlyAfterAllEvidenceAndPublicPhotoAreReady(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	reader := new(diagnosticReaderMock)
	photos := new(profilePhotoURLResolverMock)
	operators := new(diagnosticOperatorMock)
	writer := new(diagnosticAuditMock)
	d := &readmodel.ProviderDiagnostic{Provider: readmodel.Provider{ID: 12, ProfilePhotoFileID: "photo"}}
	read := reader.On("FindByProviderID", t.Context(), 12).Return(d, nil).Once()
	resolved := photos.On("ResolvePublicURLs", t.Context(), []string{"photo"}).Return(map[string]string{"photo": "https://cdn.example/photo"}, nil).Once().NotBefore(read)
	operator := operators.On("FindOperatorIDByAuthID", t.Context(), "subject").Return(21, nil).Once().NotBefore(resolved)
	writer.On("Save", t.Context(), mock.MatchedBy(func(e *audit.Event) bool {
		return e.ResourceID() == "12" && e.ResourceType() == "provider" && e.Action() == audit.ActionAccess && e.Result() == audit.ResultPrepared && e.OperatorID() == 21 && e.CorrelationID() == "diagnostic-1" && e.Reason() == nil && e.StateChange() == nil
	})).Return(nil).Once().NotBefore(operator)
	result, err := admin.NewDiagnosticService(reader, photos, operators, writer, diagnosticClock{now}).Query(t.Context(), "12", "subject", "diagnostic-1")
	require.NoError(t, err)
	require.Len(t, result.DiagnosticChecks, 4)
	require.Equal(t, "https://cdn.example/photo", result.Provider.ProfilePhotoURL)
	reader.AssertExpectations(t)
	photos.AssertExpectations(t)
	operators.AssertExpectations(t)
	writer.AssertExpectations(t)
}
func TestDiagnosticServiceFailsClosedBeforeAudit(t *testing.T) {
	failure := errors.New("source failed")
	for _, tc := range []struct {
		name               string
		found              *readmodel.ProviderDiagnostic
		readErr, errorWant error
		photoErr           error
		urls               map[string]string
	}{
		{name: "source", readErr: failure, errorWant: failure},
		{name: "missing", errorWant: admin.ErrProviderDiagnosticNotFound},
		{name: "photo source", found: &readmodel.ProviderDiagnostic{Provider: readmodel.Provider{ID: 12, ProfilePhotoFileID: "photo"}}, photoErr: failure, errorWant: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := new(diagnosticReaderMock)
			photos := new(profilePhotoURLResolverMock)
			operators := new(diagnosticOperatorMock)
			writer := new(diagnosticAuditMock)
			reader.On("FindByProviderID", t.Context(), 12).Return(tc.found, tc.readErr).Once()
			if tc.found != nil {
				photos.On("ResolvePublicURLs", t.Context(), []string{"photo"}).Return(tc.urls, tc.photoErr).Once()
			}
			result, err := admin.NewDiagnosticService(reader, photos, operators, writer, diagnosticClock{time.Now()}).Query(t.Context(), "12", "subject", "diagnostic-1")
			require.Nil(t, result)
			require.ErrorIs(t, err, tc.errorWant)
			writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
			operators.AssertNotCalled(t, "FindOperatorIDByAuthID", mock.Anything, mock.Anything)
		})
	}
}
func TestDiagnosticServiceRejectsInvalidIDsWithoutReading(t *testing.T) {
	for _, id := range []string{"0", "-1", "abc", "2147483648"} {
		reader := new(diagnosticReaderMock)
		result, err := admin.NewDiagnosticService(reader, nil, nil, nil, nil).Query(t.Context(), id, "subject", "request")
		require.Nil(t, result)
		require.ErrorIs(t, err, admin.ErrInvalidDiagnosticProviderID)
		reader.AssertNotCalled(t, "FindByProviderID", mock.Anything, mock.Anything)
	}
}
func TestDiagnosticServiceDoesNotDeliverWhenAuditFails(t *testing.T) {
	reader := new(diagnosticReaderMock)
	photos := new(profilePhotoURLResolverMock)
	operators := new(diagnosticOperatorMock)
	writer := new(diagnosticAuditMock)
	failure := errors.New("audit failed")
	reader.On("FindByProviderID", mock.Anything, 12).Return(&readmodel.ProviderDiagnostic{Provider: readmodel.Provider{ID: 12, ProfilePhotoFileID: "photo"}}, nil)
	photos.On("ResolvePublicURLs", mock.Anything, []string{"photo"}).Return(map[string]string{"photo": "https://cdn.example/photo"}, nil)
	operators.On("FindOperatorIDByAuthID", mock.Anything, "subject").Return(21, nil)
	writer.On("Save", mock.Anything, mock.Anything).Return(failure).Once()
	result, err := admin.NewDiagnosticService(reader, photos, operators, writer, diagnosticClock{time.Now()}).Query(t.Context(), "12", "subject", "diagnostic-1")
	require.Nil(t, result)
	require.ErrorIs(t, err, failure)
	writer.AssertExpectations(t)
}

func TestDiagnosticServiceReturnsUnavailablePublicPhotoAsNullEvidenceAfterAudit(t *testing.T) {
	reader := new(diagnosticReaderMock)
	photos := new(profilePhotoURLResolverMock)
	operators := new(diagnosticOperatorMock)
	writer := new(diagnosticAuditMock)
	reader.On("FindByProviderID", mock.Anything, 12).Return(&readmodel.ProviderDiagnostic{Provider: readmodel.Provider{ID: 12, ProfilePhotoFileID: "pending-photo"}}, nil).Once()
	photos.On("ResolvePublicURLs", mock.Anything, []string{"pending-photo"}).Return(map[string]string{}, nil).Once()
	operators.On("FindOperatorIDByAuthID", mock.Anything, "subject").Return(21, nil).Once()
	writer.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
	result, err := admin.NewDiagnosticService(reader, photos, operators, writer, diagnosticClock{time.Now()}).Query(t.Context(), "12", "subject", "diagnostic-1")
	require.NoError(t, err)
	require.Empty(t, result.Provider.ProfilePhotoURL)
	writer.AssertExpectations(t)
}
