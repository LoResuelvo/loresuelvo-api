package operation

import (
	"context"
	"errors"
	"fmt"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/google/uuid"
)

var ErrInvalidOperationImageID = errors.New("invalid operation image ID")
var ErrOperationImageNotFound = errors.New("operation image not found")

// MediaAssociationReader resolves an image's role within one stable operation.
// An empty purpose means that the image is not associated with that operation.
type MediaAssociationReader interface {
	FindAssociatedImagePurpose(ctx context.Context, id readmodel.ID, fileID string) (string, error)
}

type MediaFileFinder interface {
	FindByID(ctx context.Context, id string) (*filedomain.File, error)
}

type MediaObjectReader interface {
	ReadObject(ctx context.Context, object filedomain.ObjectToDownload) ([]byte, error)
}

type OperationImage struct {
	Bytes    []byte
	MimeType string
}

type MediaService struct {
	association MediaAssociationReader
	files       MediaFileFinder
	storage     MediaObjectReader
}

func NewMediaService(association MediaAssociationReader, files MediaFileFinder, storage MediaObjectReader) *MediaService {
	return &MediaService{association: association, files: files, storage: storage}
}

// The two supported private-image upload policies both cap files at 5 MiB.
const maxOperationImageBytes = 5 * 1024 * 1024

func (service *MediaService) Get(ctx context.Context, rawOperationID, rawFileID string) (*OperationImage, error) {
	id, err := ParseOperationID(rawOperationID)
	if err != nil {
		return nil, err
	}
	fileID, err := uuid.Parse(rawFileID)
	if err != nil || fileID == uuid.Nil || rawFileID != fileID.String() {
		return nil, ErrInvalidOperationImageID
	}
	canonicalFileID := fileID.String()
	purpose, err := service.association.FindAssociatedImagePurpose(ctx, id, canonicalFileID)
	if err != nil {
		return nil, fmt.Errorf("finding operation image association: %w", err)
	}
	if purpose != filedomain.PurposeJobRequestImage && purpose != filedomain.PurposeWorkOrderCompletionImage {
		return nil, ErrOperationImageNotFound
	}
	file, err := service.files.FindByID(ctx, canonicalFileID)
	if err != nil {
		return nil, fmt.Errorf("finding operation image file: %w", err)
	}
	if file == nil || file.Purpose != purpose || !file.IsConfirmed() || file.Visibility != filedomain.VisibilityPrivate ||
		file.SizeBytes() <= 0 || file.SizeBytes() > maxOperationImageBytes || !operationImageMimeAllowed(file.MimeType()) {
		return nil, ErrOperationImageNotFound
	}
	data, err := service.storage.ReadObject(ctx, filedomain.ObjectToDownload{
		Bucket: file.Bucket, Key: file.Key, MaxSizeBytes: file.SizeBytes(),
	})
	if err != nil {
		return nil, fmt.Errorf("reading operation image object: %w", err)
	}
	if len(data) != file.SizeBytes() {
		return nil, fmt.Errorf("reading operation image object: size mismatch")
	}
	return &OperationImage{Bytes: data, MimeType: file.MimeType()}, nil
}

func operationImageMimeAllowed(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}
