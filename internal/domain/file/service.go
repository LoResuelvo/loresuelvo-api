package file

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repository    Repository
	storage       Storage
	publicBucket  string
	privateBucket string
	clock         Clock
	idGenerator   func() string
	policies      map[string]UploadPolicy
}

func NewService(repository Repository, storage Storage, publicBucket, privateBucket string, clock Clock) *Service {
	if clock == nil {
		panic("file clock is required")
	}

	return &Service{
		repository:    repository,
		storage:       storage,
		publicBucket:  publicBucket,
		privateBucket: privateBucket,
		clock:         clock,
		idGenerator:   uuid.NewString,
		policies:      defaultUploadPolicies(),
	}
}

func (s *Service) RequestUpload(ctx context.Context, request PresignRequest) (*PresignUploadResult, error) {
	policy, err := s.policyFor(request.Purpose)
	if err != nil {
		return nil, err
	}
	metadata, err := validateUploadRequest(request, policy)
	if err != nil {
		return nil, err
	}

	createdOn := s.clock.Now()
	fileID := s.idGenerator()
	bucket := s.bucketForVisibility(policy.Visibility)
	key := buildObjectKey(createdOn, request.Purpose, fileID, metadata.OriginalName())

	target, err := s.storage.GenerateUploadURL(ctx, ObjectToUpload{
		Bucket:    bucket,
		Key:       key,
		MimeType:  metadata.MimeType(),
		SizeBytes: metadata.SizeBytes(),
	})
	if err != nil {
		return nil, fmt.Errorf("generating upload url: %w", err)
	}

	file, err := NewPendingFile(fileID, key, bucket, *metadata, policy.Visibility, request.Purpose, request.AuthID, createdOn)
	if err != nil {
		return nil, err
	}
	if err := s.repository.Save(ctx, *file); err != nil {
		return nil, fmt.Errorf("saving pending file: %w", err)
	}

	return &PresignUploadResult{
		FileID:  fileID,
		Key:     key,
		URL:     target.URL,
		Headers: target.Headers,
	}, nil
}

func (s *Service) ConfirmUpload(ctx context.Context, request ConfirmRequest) (*ConfirmUploadResult, error) {
	file, err := s.repository.FindByID(ctx, request.FileID)
	if err != nil {
		return nil, ErrProfilePhotoNotAvailable
	}

	if file.UploadedByAuthID != request.AuthID || file.Key != request.Key {
		return nil, ErrProfilePhotoNotAvailable
	}
	if file.MimeType() != request.MimeType || file.SizeBytes() != request.SizeBytes {
		return nil, ErrProfilePhotoNotAvailable
	}

	metadata, err := s.storage.ReadObjectMetadata(ctx, file.Bucket, file.Key)
	if err != nil {
		return nil, ErrProfilePhotoNotAvailable
	}
	if metadata.MimeType != file.MimeType() || metadata.SizeBytes != file.SizeBytes() {
		return nil, ErrProfilePhotoNotAvailable
	}

	file.Confirm(s.clock.Now())
	if err := s.repository.Save(ctx, *file); err != nil {
		return nil, fmt.Errorf("confirming file upload: %w", err)
	}

	return &ConfirmUploadResult{
		FileID:       file.ID,
		URL:          s.publicURL(*file),
		OriginalName: file.OriginalName(),
	}, nil
}

func (s *Service) ValidateProviderProfilePhoto(ctx context.Context, authID, fileID string) error {
	if fileID == "" {
		return ErrProfilePhotoRequired
	}

	file, err := s.repository.FindByID(ctx, fileID)
	if err != nil {
		return ErrProfilePhotoNotAvailable
	}
	if !isValidProviderProfilePhotoFor(*file, authID) {
		return ErrProfilePhotoNotAvailable
	}

	return nil
}

func (s *Service) ResolvePublicURL(ctx context.Context, fileID string) (string, error) {
	urlsByID, err := s.ResolvePublicURLs(ctx, []string{fileID})
	if err != nil {
		return "", err
	}

	return urlsByID[fileID], nil
}

func (s *Service) ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error) {
	uniqueFileIDs := uniqueNonEmptyFileIDs(fileIDs)
	if len(uniqueFileIDs) == 0 {
		return map[string]string{}, nil
	}

	files, err := s.repository.FindByIDs(ctx, uniqueFileIDs)
	if err != nil {
		return nil, fmt.Errorf("finding public files: %w", err)
	}

	urlsByID := make(map[string]string, len(files))
	for _, file := range files {
		if !file.IsConfirmed() || !file.IsPublic() {
			continue
		}
		urlsByID[file.ID] = s.publicURL(file)
	}

	return urlsByID, nil
}

func (s *Service) policyFor(purpose string) (UploadPolicy, error) {
	if purpose == "" {
		return UploadPolicy{}, ErrPurposeRequired
	}
	policy, ok := s.policies[purpose]
	if !ok {
		return UploadPolicy{}, ErrUnsupportedPurpose
	}
	return policy, nil
}

func (s *Service) bucketForVisibility(visibility string) string {
	if visibility == VisibilityPublic {
		return s.publicBucket
	}
	return s.privateBucket
}

func (s *Service) publicURL(file File) string {
	return s.storage.PublicURL(file.Bucket, file.Key)
}

func uniqueNonEmptyFileIDs(fileIDs []string) []string {
	seen := make(map[string]struct{}, len(fileIDs))
	unique := make([]string, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		if fileID == "" {
			continue
		}
		if _, ok := seen[fileID]; ok {
			continue
		}
		seen[fileID] = struct{}{}
		unique = append(unique, fileID)
	}
	return unique
}

func isValidProviderProfilePhotoFor(file File, authID string) bool {
	return file.IsConfirmed() &&
		file.IsPublic() &&
		file.WasUploadedBy(authID) &&
		file.HasPurpose(PurposeProviderProfilePhoto) &&
		providerProfilePhotoPolicy.Allows(file.Metadata())
}
