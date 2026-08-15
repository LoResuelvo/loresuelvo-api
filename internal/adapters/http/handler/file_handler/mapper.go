package file_handler

import (
	"strings"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

func presignRequestFromHTTP(authID string, req presignFileRequest) filedomain.PresignRequest {
	return filedomain.PresignRequest{
		AuthID:       authID,
		OriginalName: strings.TrimSpace(req.OriginalName),
		MimeType:     strings.TrimSpace(strings.ToLower(req.MimeType)),
		SizeBytes:    req.SizeBytes,
		Purpose:      strings.TrimSpace(req.Purpose),
	}
}

func confirmRequestFromHTTP(authID string, fileID string, req confirmFileRequest) filedomain.ConfirmRequest {
	return filedomain.ConfirmRequest{
		AuthID:    authID,
		FileID:    strings.TrimSpace(fileID),
		Key:       strings.TrimSpace(req.Key),
		MimeType:  strings.TrimSpace(strings.ToLower(req.MimeType)),
		SizeBytes: req.SizeBytes,
	}
}

func presignFileResponseFromDomain(result *filedomain.PresignUploadResult) presignFileResponse {
	return presignFileResponse{
		FileID:    result.FileID,
		Key:       result.Key,
		UploadURL: result.URL,
		Headers:   result.Headers,
	}
}

func fileResponseFromDomain(file *filedomain.ConfirmUploadResult) fileResponse {
	response := fileResponse{
		ID:           file.FileID,
		URL:          file.URL,
		OriginalName: file.OriginalName,
		MimeType:     file.MimeType,
		Type:         fileResponseType(file),
	}

	switch response.Type {
	case fileTypeAudio:
		response.Audio = &fileAudioResponse{
			Codec:           file.Codec,
			DurationSeconds: file.DurationSeconds,
		}
	case fileTypeVideo:
		response.Video = &fileVideoResponse{
			VideoCodec:      file.VideoCodec,
			AudioCodec:      file.AudioCodec,
			DurationSeconds: file.DurationSeconds,
			Width:           file.Width,
			Height:          file.Height,
		}
	}
	return response
}

const (
	fileTypeImage = "image"
	fileTypeAudio = "audio"
	fileTypeVideo = "video"
)

func fileResponseType(file *filedomain.ConfirmUploadResult) string {
	if file.VideoCodec != "" {
		return fileTypeVideo
	}
	if file.Codec != "" {
		return fileTypeAudio
	}
	return fileTypeImage
}
