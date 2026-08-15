package file

import "strings"

type Metadata interface {
	OriginalName() string
	MimeType() string
	SizeBytes() int
	DurationSeconds() int
	Codec() string
	VideoCodec() string
	AudioCodec() string
	Width() int
	Height() int
}

type FileMetadata struct {
	originalName string
	mimeType     string
	sizeBytes    int
}

type AudioFileMetadata struct {
	FileMetadata
	durationSeconds int
	codec           string
}

type VideoFileMetadata struct {
	FileMetadata
	durationSeconds int
	videoCodec      string
	audioCodec      string
	width           int
	height          int
}

type VideoMetadata struct {
	DurationSeconds int
	VideoCodec      string
	AudioCodec      string
	Width           int
	Height          int
}

func NewFileMetadata(originalName, mimeType string, sizeBytes int) (*FileMetadata, error) {
	if originalName == "" {
		return nil, ErrOriginalNameRequired
	}
	if mimeType == "" {
		return nil, ErrMimeTypeRequired
	}
	if sizeBytes <= 0 {
		return nil, ErrSizeRequired
	}

	return &FileMetadata{
		originalName: originalName,
		mimeType:     mimeType,
		sizeBytes:    sizeBytes,
	}, nil
}

func NewAudioFileMetadata(originalName, mimeType string, sizeBytes, durationSeconds int, codec string) (*AudioFileMetadata, error) {
	baseMetadata, err := NewFileMetadata(originalName, mimeType, sizeBytes)
	if err != nil {
		return nil, err
	}
	if durationSeconds <= 0 {
		return nil, ErrAudioDurationRequired
	}

	normalizedCodec := strings.ToLower(strings.TrimSpace(codec))
	if normalizedCodec == "" {
		return nil, ErrAudioCodecRequired
	}

	return &AudioFileMetadata{
		FileMetadata:    *baseMetadata,
		durationSeconds: durationSeconds,
		codec:           normalizedCodec,
	}, nil
}

func (m FileMetadata) OriginalName() string {
	return m.originalName
}

func (m FileMetadata) MimeType() string {
	return m.mimeType
}

func (m FileMetadata) SizeBytes() int {
	return m.sizeBytes
}

func (m FileMetadata) DurationSeconds() int {
	return 0
}

func (m FileMetadata) Codec() string {
	return ""
}

func (m FileMetadata) VideoCodec() string {
	return ""
}

func (m FileMetadata) AudioCodec() string {
	return ""
}

func (m FileMetadata) Width() int {
	return 0
}

func (m FileMetadata) Height() int {
	return 0
}

func (m AudioFileMetadata) DurationSeconds() int {
	return m.durationSeconds
}

func (m AudioFileMetadata) Codec() string {
	return m.codec
}

func NewVideoFileMetadata(originalName, mimeType string, sizeBytes int, metadata VideoMetadata) (*VideoFileMetadata, error) {
	baseMetadata, err := NewFileMetadata(originalName, mimeType, sizeBytes)
	if err != nil {
		return nil, err
	}
	if metadata.DurationSeconds <= 0 {
		return nil, ErrVideoDurationRequired
	}

	normalizedVideoCodec := strings.ToLower(strings.TrimSpace(metadata.VideoCodec))
	if normalizedVideoCodec == "" {
		return nil, ErrVideoCodecRequired
	}
	if metadata.Width <= 0 || metadata.Height <= 0 {
		return nil, ErrVideoDimensionsRequired
	}

	return &VideoFileMetadata{
		FileMetadata:    *baseMetadata,
		durationSeconds: metadata.DurationSeconds,
		videoCodec:      normalizedVideoCodec,
		audioCodec:      strings.ToLower(strings.TrimSpace(metadata.AudioCodec)),
		width:           metadata.Width,
		height:          metadata.Height,
	}, nil
}

func (m VideoFileMetadata) DurationSeconds() int {
	return m.durationSeconds
}

func (m VideoFileMetadata) VideoCodec() string {
	return m.videoCodec
}

func (m VideoFileMetadata) AudioCodec() string {
	return m.audioCodec
}

func (m VideoFileMetadata) Width() int {
	return m.width
}

func (m VideoFileMetadata) Height() int {
	return m.height
}
