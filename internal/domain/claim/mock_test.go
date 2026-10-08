package claim

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
)

type repositoryMock struct{ mock.Mock }

func (m *repositoryMock) Save(ctx context.Context, c *Claim) error { return m.Called(ctx, c).Error(0) }
func (m *repositoryMock) FindBySubmissionKey(ctx context.Context, id int, key string) (*Claim, error) {
	a := m.Called(ctx, id, key)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*Claim), a.Error(1)
}
func (m *repositoryMock) FindOwnedByID(ctx context.Context, id, claimID int) (*Claim, error) {
	a := m.Called(ctx, id, claimID)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*Claim), a.Error(1)
}
func (m *repositoryMock) FindOwnedPage(ctx context.Context, id int, criteria ListCriteria) (*Page, error) {
	a := m.Called(ctx, id, criteria)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*Page), a.Error(1)
}

type userFinderMock struct{ mock.Mock }

func (m *userFinderMock) FindClaimantByAuthID(ctx context.Context, id string) (*Claimant, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*Claimant), a.Error(1)
}

type operationReferenceResolverMock struct{ mock.Mock }

func (m *operationReferenceResolverMock) ResolveClaimOperationReference(ctx context.Context, c Claimant, r Reference) (*operationmodel.ID, error) {
	a := m.Called(ctx, c, r)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*operationmodel.ID), a.Error(1)
}

type evidenceImagesMock struct{ mock.Mock }

func (m *evidenceImagesMock) ValidateClaimEvidenceImages(ctx context.Context, id string, files []string) error {
	return m.Called(ctx, id, files).Error(0)
}
func (m *evidenceImagesMock) ResolveClaimEvidenceImage(ctx context.Context, id, fileID string) (string, error) {
	a := m.Called(ctx, id, fileID)
	return a.String(0), a.Error(1)
}

type clockMock struct{ mock.Mock }

func (m *clockMock) Now() time.Time { return m.Called().Get(0).(time.Time) }

type adminReaderMock struct{ mock.Mock }

func (m *adminReaderMock) FindAdministrativePage(ctx context.Context, criteria AdminCriteria) (*AdminPage, error) {
	a := m.Called(ctx, criteria)
	v, _ := a.Get(0).(*AdminPage)
	return v, a.Error(1)
}
func (m *adminReaderMock) FindAdministrativeByID(ctx context.Context, id int) (*AdminDetail, error) {
	a := m.Called(ctx, id)
	v, _ := a.Get(0).(*AdminDetail)
	return v, a.Error(1)
}

type operatorFinderMock struct{ mock.Mock }

func (m *operatorFinderMock) FindOperatorIDByAuthID(ctx context.Context, auth string) (int, error) {
	a := m.Called(ctx, auth)
	return a.Int(0), a.Error(1)
}

type auditWriterMock struct{ mock.Mock }

func (m *auditWriterMock) Save(ctx context.Context, event *audit.Event) error {
	return m.Called(ctx, event).Error(0)
}

type administrativeEvidenceImagesMock struct{ mock.Mock }

func (m *administrativeEvidenceImagesMock) ResolveAdministrativeClaimEvidenceImage(ctx context.Context, id string) (string, error) {
	a := m.Called(ctx, id)
	return a.String(0), a.Error(1)
}

type administrationUnitOfWorkMock struct{ mock.Mock }

func (m *administrationUnitOfWorkMock) Execute(ctx context.Context, operation func(AdministrationStore) error) error {
	return m.Called(ctx, operation).Error(0)
}

type administrationStoreMock struct{ mock.Mock }

func (m *administrationStoreMock) FindClaim(ctx context.Context, id int) (*Claim, error) {
	a := m.Called(ctx, id)
	v, _ := a.Get(0).(*Claim)
	return v, a.Error(1)
}
func (m *administrationStoreMock) FindRecord(ctx context.Context, id int, key string) (*AdministrationRecord, error) {
	a := m.Called(ctx, id, key)
	v, _ := a.Get(0).(*AdministrationRecord)
	return v, a.Error(1)
}
func (m *administrationStoreMock) SaveClaim(ctx context.Context, c *Claim) error {
	return m.Called(ctx, c).Error(0)
}
func (m *administrationStoreMock) SaveRecord(ctx context.Context, record *AdministrationRecord) error {
	return m.Called(ctx, record).Error(0)
}
func (m *administrationStoreMock) SaveAuditEvent(ctx context.Context, event *audit.Event) error {
	return m.Called(ctx, event).Error(0)
}
