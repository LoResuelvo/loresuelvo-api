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

func (s *Service) ValidateMessageImages(ctx context.Context, authID string, fileIDs []string) ([]MessageImage, error) {
	files, err := s.validatedMessageImageFiles(ctx, authID, fileIDs)
	if err != nil {
		return nil, err
	}
	result := make([]MessageImage, 0, len(files))
	for _, file := range files {
		result = append(result, MessageImage{FileID: file.ID, OriginalName: file.OriginalName()})
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
	if len(fileIDs) == 0 {
		return []File{}, nil
	}
	if len(fileIDs) > MaxConversationMessageImages {
		return nil, ErrMessageImageNotAvailable
	}
	uniqueFileIDs := uniqueNonEmptyFileIDs(fileIDs)
	if len(uniqueFileIDs) != len(fileIDs) {
		return nil, ErrMessageImageNotAvailable
	}

	files, err := s.repository.FindByIDs(ctx, uniqueFileIDs)
	if err != nil {
		return nil, fmt.Errorf("finding message images for validation: %w", err)
	}
	if len(files) != len(uniqueFileIDs) {
		return nil, ErrMessageImageNotAvailable
	}
	filesByID := make(map[string]File, len(files))
	for _, file := range files {
		if !isValidConversationMessageImageFor(file, authID) {
			return nil, ErrMessageImageNotAvailable
		}
		filesByID[file.ID] = file
	}

	result := make([]File, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		file, ok := filesByID[fileID]
		if !ok {
			return nil, ErrMessageImageNotAvailable
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
		if !isAvailableConversationMessageImage(file) {
			continue
		}
		resolved, err := s.resolveMessageImage(ctx, file)
		if err != nil {
			return nil, err
		}
		result[file.ID] = resolved
	}
	return result, nil
}

func (s *Service) resolveMessageImage(ctx context.Context, file File) (MessageImage, error) {
	url, err := s.storage.GenerateDownloadURL(ctx, ObjectToDownload{Bucket: file.Bucket, Key: file.Key})
	if err != nil {
		return MessageImage{}, fmt.Errorf("generating message image download url: %w", err)
	}
	return MessageImage{FileID: file.ID, OriginalName: file.OriginalName(), URL: url}, nil
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

func isValidConversationMessageImageFor(file File, authID string) bool {
	return isAvailableConversationMessageImage(file) && file.WasUploadedBy(authID)
}

func isAvailableConversationMessageImage(file File) bool {
	return file.IsConfirmed() &&
		!file.IsPublic() &&
		file.HasPurpose(PurposeConversationMessageImage) &&
		conversationMessageImagePolicy.Allows(file.Metadata())
}
