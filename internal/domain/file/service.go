package file

import (
	"context"
	"fmt"

	domainclock "github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
)

type Service struct {
	repository    Repository
	storage       Storage
	publicBucket  string
	privateBucket string
	clock         domainclock.Clock
	idGenerator   func() string
	policies      map[string]UploadPolicy
}

type imageValidation struct {
	policy       UploadPolicy
	maxFiles     int
	errorContext string
}

var conversationMessageImageValidation = imageValidation{
	policy:       conversationMessageImagePolicy,
	maxFiles:     MaxConversationMessageImages,
	errorContext: "message",
}

var jobRequestImageValidation = imageValidation{
	policy:       jobRequestImagePolicy,
	maxFiles:     MaxJobRequestImages,
	errorContext: "job request",
}

func NewService(repository Repository, storage Storage, publicBucket, privateBucket string, clock domainclock.Clock) *Service {
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
		return nil, ErrFileNotAvailable
	}

	if file.UploadedByAuthID != request.AuthID || file.Key != request.Key {
		return nil, ErrFileNotAvailable
	}
	if file.MimeType() != request.MimeType || file.SizeBytes() != request.SizeBytes {
		return nil, ErrFileNotAvailable
	}

	metadata, err := s.storage.ReadObjectMetadata(ctx, file.Bucket, file.Key)
	if err != nil {
		return nil, ErrFileNotAvailable
	}
	if metadata.MimeType != file.MimeType() || metadata.SizeBytes != file.SizeBytes() {
		return nil, ErrFileNotAvailable
	}

	file.Confirm(s.clock.Now())
	if err := s.repository.Save(ctx, *file); err != nil {
		return nil, fmt.Errorf("confirming file upload: %w", err)
	}

	return &ConfirmUploadResult{
		FileID:       file.ID,
		URL:          s.confirmedFileURL(*file),
		OriginalName: file.OriginalName(),
	}, nil
}

func (s *Service) confirmedFileURL(file File) string {
	if !file.IsPublic() {
		return ""
	}
	return s.publicURL(file)
}

func (s *Service) ValidateProfilePhoto(ctx context.Context, authID, fileID string) error {
	file, err := s.repository.FindByID(ctx, fileID)
	if err != nil {
		return ErrProfilePhotoNotAvailable
	}
	if !isValidProfilePhotoFor(*file, authID) {
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

func (s *Service) PrepareJobRequestImages(ctx context.Context, authID string, fileIDs []string) ([]Image, error) {
	files, err := s.validatedJobRequestImageFiles(ctx, authID, fileIDs)
	if err != nil {
		return nil, err
	}
	result := make([]Image, 0, len(files))
	for _, file := range files {
		resolved, err := s.resolveImage(ctx, file)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved)
	}
	return result, nil
}

func (s *Service) PrepareMessageImages(ctx context.Context, authID string, fileIDs []string) ([]MessageImage, error) {
	files, err := s.validatedMessageImageFiles(ctx, authID, fileIDs)
	if err != nil {
		return nil, err
	}
	result := make([]MessageImage, 0, len(files))
	for _, file := range files {
		resolved, err := s.resolveMessageImage(ctx, file)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved)
	}
	return result, nil
}

func (s *Service) PrepareChatbotMessageImages(ctx context.Context, authID string, fileIDs []string) ([]MessageImageContent, error) {
	files, err := s.validatedMessageImageFiles(ctx, authID, fileIDs)
	if err != nil {
		return nil, err
	}
	result := make([]MessageImageContent, 0, len(files))
	for _, file := range files {
		resolved, err := s.resolveMessageImage(ctx, file)
		if err != nil {
			return nil, err
		}
		data, err := s.storage.ReadObject(ctx, ObjectToDownload{
			Bucket:       file.Bucket,
			Key:          file.Key,
			MaxSizeBytes: conversationMessageImagePolicy.MaxSizeBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("reading chatbot message image: %w", err)
		}
		if len(data) == 0 || len(data) != file.SizeBytes() || len(data) > conversationMessageImagePolicy.MaxSizeBytes {
			return nil, ErrMessageImageNotAvailable
		}
		result = append(result, MessageImageContent{
			MessageImage: resolved,
			MimeType:     file.MimeType(),
			Data:         data,
		})
	}
	return result, nil
}

func (s *Service) validatedMessageImageFiles(ctx context.Context, authID string, fileIDs []string) ([]File, error) {
	return s.validatedImageFiles(ctx, authID, fileIDs, conversationMessageImageValidation)
}

func (s *Service) validatedJobRequestImageFiles(ctx context.Context, authID string, fileIDs []string) ([]File, error) {
	return s.validatedImageFiles(ctx, authID, fileIDs, jobRequestImageValidation)
}

func (s *Service) validatedImageFiles(ctx context.Context, authID string, fileIDs []string, validation imageValidation) ([]File, error) {
	if len(fileIDs) == 0 {
		return []File{}, nil
	}
	if len(fileIDs) > validation.maxFiles {
		return nil, validation.policy.InvalidMetadataError
	}
	uniqueFileIDs := uniqueNonEmptyFileIDs(fileIDs)
	if len(uniqueFileIDs) != len(fileIDs) {
		return nil, validation.policy.InvalidMetadataError
	}

	files, err := s.repository.FindByIDs(ctx, uniqueFileIDs)
	if err != nil {
		return nil, fmt.Errorf("finding %s images for validation: %w", validation.errorContext, err)
	}
	if len(files) != len(uniqueFileIDs) {
		return nil, validation.policy.InvalidMetadataError
	}
	filesByID := make(map[string]File, len(files))
	for _, file := range files {
		if !isValidImageFor(file, authID, validation.policy) {
			return nil, validation.policy.InvalidMetadataError
		}
		filesByID[file.ID] = file
	}

	result := make([]File, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		file, ok := filesByID[fileID]
		if !ok {
			return nil, validation.policy.InvalidMetadataError
		}
		result = append(result, file)
	}
	return result, nil
}

func (s *Service) ResolveMessageImages(ctx context.Context, fileIDs []string) (map[string]MessageImage, error) {
	uniqueFileIDs := uniqueNonEmptyFileIDs(fileIDs)
	if len(uniqueFileIDs) == 0 {
		return map[string]MessageImage{}, nil
	}
	files, err := s.repository.FindByIDs(ctx, uniqueFileIDs)
	if err != nil {
		return nil, fmt.Errorf("finding message images: %w", err)
	}

	result := make(map[string]MessageImage, len(files))
	for _, file := range files {
		if !isAvailableImageForPolicy(file, conversationMessageImagePolicy) {
			continue
		}
		resolved, err := s.resolveImage(ctx, file)
		if err != nil {
			return nil, err
		}
		result[file.ID] = MessageImage{Image: resolved}
	}
	return result, nil
}

func (s *Service) resolveMessageImage(ctx context.Context, file File) (MessageImage, error) {
	resolved, err := s.resolveImage(ctx, file)
	if err != nil {
		return MessageImage{}, err
	}
	return MessageImage{Image: resolved}, nil
}

func (s *Service) ResolveJobRequestImages(ctx context.Context, images []Image) ([]Image, error) {
	fileIDs := make([]string, 0, len(images))
	for _, image := range images {
		fileIDs = append(fileIDs, image.FileID)
	}
	uniqueFileIDs := uniqueNonEmptyFileIDs(fileIDs)
	if len(uniqueFileIDs) != len(fileIDs) {
		return nil, ErrJobRequestImageNotAvailable
	}
	if len(uniqueFileIDs) == 0 {
		return []Image{}, nil
	}

	files, err := s.repository.FindByIDs(ctx, uniqueFileIDs)
	if err != nil {
		return nil, fmt.Errorf("finding job request images: %w", err)
	}
	if len(files) != len(uniqueFileIDs) {
		return nil, ErrJobRequestImageNotAvailable
	}
	filesByID := make(map[string]File, len(files))
	for _, file := range files {
		if !isAvailableImageForAnyPolicy(file, jobRequestImagePolicy, conversationMessageImagePolicy) {
			return nil, ErrJobRequestImageNotAvailable
		}
		filesByID[file.ID] = file
	}

	result := make([]Image, 0, len(images))
	for _, image := range images {
		file, ok := filesByID[image.FileID]
		if !ok {
			return nil, ErrJobRequestImageNotAvailable
		}
		resolved, err := s.resolveImage(ctx, file)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved)
	}
	return result, nil
}

func (s *Service) resolveImage(ctx context.Context, file File) (Image, error) {
	url, err := s.storage.GenerateDownloadURL(ctx, ObjectToDownload{Bucket: file.Bucket, Key: file.Key})
	if err != nil {
		return Image{}, fmt.Errorf("generating image download url: %w", err)
	}
	return Image{FileID: file.ID, OriginalName: file.OriginalName(), URL: url}, nil
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

func isValidProfilePhotoFor(file File, authID string) bool {
	return isValidImageFor(file, authID, profilePhotoPolicy)
}

func isValidImageFor(file File, authID string, policy UploadPolicy) bool {
	return isAvailableImageForPolicy(file, policy) && file.WasUploadedBy(authID)
}

func isAvailableImageForPolicy(file File, policy UploadPolicy) bool {
	return file.IsConfirmed() &&
		file.Visibility == policy.Visibility &&
		file.HasPurpose(policy.Purpose) &&
		policy.Allows(file.Metadata())
}

func isAvailableImageForAnyPolicy(file File, policies ...UploadPolicy) bool {
	for _, policy := range policies {
		if isAvailableImageForPolicy(file, policy) {
			return true
		}
	}
	return false
}
