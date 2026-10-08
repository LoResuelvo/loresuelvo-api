package claim_handler

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
)

type serviceMock struct{ mock.Mock }

func (m *serviceMock) Submit(ctx context.Context, auth, key string, input claim.Submission) (*claim.SubmissionResult, error) {
	a := m.Called(ctx, auth, key, input)
	result, _ := a.Get(0).(*claim.SubmissionResult)
	return result, a.Error(1)
}
func (m *serviceMock) List(ctx context.Context, auth string, criteria claim.ListCriteria) (*claim.Page, error) {
	a := m.Called(ctx, auth, criteria)
	result, _ := a.Get(0).(*claim.Page)
	return result, a.Error(1)
}
func (m *serviceMock) Get(ctx context.Context, auth string, id int) (*claim.GetResult, error) {
	a := m.Called(ctx, auth, id)
	result, _ := a.Get(0).(*claim.GetResult)
	return result, a.Error(1)
}

type claimRepositoryMock struct{ mock.Mock }

func (m *claimRepositoryMock) Save(ctx context.Context, found *claim.Claim) error {
	return m.Called(ctx, found).Error(0)
}
func (m *claimRepositoryMock) FindBySubmissionKey(ctx context.Context, id int, key string) (*claim.Claim, error) {
	a := m.Called(ctx, id, key)
	v, _ := a.Get(0).(*claim.Claim)
	return v, a.Error(1)
}
func (m *claimRepositoryMock) FindOwnedByID(ctx context.Context, id, claimID int) (*claim.Claim, error) {
	a := m.Called(ctx, id, claimID)
	v, _ := a.Get(0).(*claim.Claim)
	return v, a.Error(1)
}
func (m *claimRepositoryMock) FindOwnedPage(ctx context.Context, id int, c claim.ListCriteria) (*claim.Page, error) {
	a := m.Called(ctx, id, c)
	v, _ := a.Get(0).(*claim.Page)
	return v, a.Error(1)
}

type claimantFinderMock struct{ mock.Mock }

func (m *claimantFinderMock) FindClaimantByAuthID(ctx context.Context, auth string) (*claim.Claimant, error) {
	a := m.Called(ctx, auth)
	v, _ := a.Get(0).(*claim.Claimant)
	return v, a.Error(1)
}

type operationResolverMock struct{ mock.Mock }

func (m *operationResolverMock) ResolveClaimOperationReference(ctx context.Context, c claim.Claimant, r claim.Reference) (*operationmodel.ID, error) {
	a := m.Called(ctx, c, r)
	v, _ := a.Get(0).(*operationmodel.ID)
	return v, a.Error(1)
}

type fileRepositoryMock struct{ mock.Mock }

func (m *fileRepositoryMock) Save(ctx context.Context, f filedomain.File) error {
	return m.Called(ctx, f).Error(0)
}
func (m *fileRepositoryMock) FindByID(ctx context.Context, id string) (*filedomain.File, error) {
	a := m.Called(ctx, id)
	v, _ := a.Get(0).(*filedomain.File)
	return v, a.Error(1)
}
func (m *fileRepositoryMock) FindByIDs(ctx context.Context, ids []string) ([]filedomain.File, error) {
	a := m.Called(ctx, ids)
	v, _ := a.Get(0).([]filedomain.File)
	return v, a.Error(1)
}
func (m *fileRepositoryMock) DeleteAll() error { return m.Called().Error(0) }

type storageMock struct{ mock.Mock }

func (m *storageMock) GenerateUploadURL(ctx context.Context, o filedomain.ObjectToUpload) (*filedomain.UploadTarget, error) {
	a := m.Called(ctx, o)
	v, _ := a.Get(0).(*filedomain.UploadTarget)
	return v, a.Error(1)
}
func (m *storageMock) GenerateDownloadURL(ctx context.Context, o filedomain.ObjectToDownload) (string, error) {
	a := m.Called(ctx, o)
	return a.String(0), a.Error(1)
}
func (m *storageMock) ReadObjectMetadata(ctx context.Context, bucket, key string) (*filedomain.ObjectMetadata, error) {
	a := m.Called(ctx, bucket, key)
	v, _ := a.Get(0).(*filedomain.ObjectMetadata)
	return v, a.Error(1)
}
func (m *storageMock) ReadObject(ctx context.Context, o filedomain.ObjectToDownload) ([]byte, error) {
	a := m.Called(ctx, o)
	v, _ := a.Get(0).([]byte)
	return v, a.Error(1)
}
func (m *storageMock) PublicURL(bucket, key string) string { return m.Called(bucket, key).String(0) }

func newServiceMock(t *testing.T) *serviceMock {
	t.Helper()
	m := new(serviceMock)
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

type adminServiceMock struct{ mock.Mock }

func (m *adminServiceMock) List(ctx context.Context, criteria claim.AdminCriteria) (*claim.AdminPage, error) {
	a := m.Called(ctx, criteria)
	v, _ := a.Get(0).(*claim.AdminPage)
	return v, a.Error(1)
}
func (m *adminServiceMock) Get(ctx context.Context, auth string, id int, correlation string) (*claim.AdminDetail, error) {
	a := m.Called(ctx, auth, id, correlation)
	v, _ := a.Get(0).(*claim.AdminDetail)
	return v, a.Error(1)
}
func (m *adminServiceMock) StartReview(ctx context.Context, auth string, id int, key, correlation string) (*claim.AdministrationResult, error) {
	a := m.Called(ctx, auth, id, key, correlation)
	v, _ := a.Get(0).(*claim.AdministrationResult)
	return v, a.Error(1)
}
func (m *adminServiceMock) Resolve(ctx context.Context, auth string, id int, key, correlation string, input claim.ResolutionInput) (*claim.AdministrationResult, error) {
	a := m.Called(ctx, auth, id, key, correlation, input)
	v, _ := a.Get(0).(*claim.AdministrationResult)
	return v, a.Error(1)
}
