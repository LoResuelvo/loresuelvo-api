package file

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/google/uuid"
)

var claimEvidenceImageValidation = imageValidation{
	policy:       claimEvidenceImagePolicy,
	maxFiles:     MaxClaimEvidenceImages,
	errorContext: "claim evidence",
}

func (s *Service) ValidateClaimEvidenceImages(ctx context.Context, authID string, fileIDs []string) error {
	for _, id := range fileIDs {
		if _, err := uuid.Parse(id); err != nil {
			return ErrClaimEvidenceImageNotAvailable
		}
	}
	_, err := s.validatedImageFiles(ctx, authID, fileIDs, claimEvidenceImageValidation)
	return err
}

// ResolveClaimEvidenceImage must only be called after authorizing the claim and its attachment.
func (s *Service) ResolveClaimEvidenceImage(ctx context.Context, authID, fileID string) (string, error) {
	if _, err := uuid.Parse(fileID); err != nil {
		return "", ErrClaimEvidenceImageNotAvailable
	}
	files, err := s.validatedImageFiles(ctx, authID, []string{fileID}, claimEvidenceImageValidation)
	if err != nil {
		return "", err
	}
	resolved, err := s.resolveImage(ctx, files[0])
	return resolved.URL, err
}

func (s *Service) confirmClaimEvidenceImage(ctx context.Context, file *File) error {
	if file.Visibility != VisibilityPrivate || !claimEvidenceImagePolicy.Allows(file.Metadata()) {
		return ErrClaimEvidenceImageNotAvailable
	}
	data, err := s.storage.ReadObject(ctx, ObjectToDownload{Bucket: file.Bucket, Key: file.Key, MaxSizeBytes: claimEvidenceImagePolicy.MaxSizeBytes})
	if err != nil {
		return fmt.Errorf("reading claim evidence image: %w", err)
	}
	if len(data) != file.SizeBytes() || len(data) > claimEvidenceImagePolicy.MaxSizeBytes {
		return ErrClaimEvidenceImageNotAvailable
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || "image/"+format != file.MimeType() {
		return ErrClaimEvidenceImageNotAvailable
	}
	file.Confirm(s.clock.Now())
	return nil
}

// ResolveAdministrativeClaimEvidenceImage is an explicit capability for an authorized,
// audited administrative claim detail. Its caller must restrict access to linked file IDs.
func (s *Service) ResolveAdministrativeClaimEvidenceImage(ctx context.Context, fileID string) (string, error) {
	if _, err := uuid.Parse(fileID); err != nil {
		return "", ErrClaimEvidenceImageNotAvailable
	}
	files, err := s.repository.FindByIDs(ctx, []string{fileID})
	if err != nil {
		return "", fmt.Errorf("finding administrative claim evidence: %w", err)
	}
	if len(files) != 1 || files[0].ID != fileID || !isAvailableImageForPolicy(files[0], claimEvidenceImagePolicy) {
		return "", ErrClaimEvidenceImageNotAvailable
	}
	resolved, err := s.resolveImage(ctx, files[0])
	if err != nil {
		return "", err
	}
	return resolved.URL, nil
}
