package file_handler

import (
	"encoding/json"
	"testing"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileResponseFromDomainUsesCommonAndVariantFields(t *testing.T) {
	tests := []struct {
		name       string
		file       filedomain.ConfirmUploadResult
		expectType string
		assertBody func(t *testing.T, response fileResponse)
	}{
		{
			name: "image",
			file: filedomain.ConfirmUploadResult{
				FileID:       "image-id",
				OriginalName: "photo.jpg",
				MimeType:     "image/jpeg",
			},
			expectType: fileTypeImage,
			assertBody: func(t *testing.T, response fileResponse) {
				assert.Nil(t, response.Audio)
				assert.Nil(t, response.Video)
			},
		},
		{
			name: "audio",
			file: filedomain.ConfirmUploadResult{
				FileID:          "audio-id",
				OriginalName:    "voice.webm",
				MimeType:        "audio/webm",
				Codec:           "opus",
				DurationSeconds: 18,
			},
			expectType: fileTypeAudio,
			assertBody: func(t *testing.T, response fileResponse) {
				require.NotNil(t, response.Audio)
				assert.Equal(t, fileAudioResponse{Codec: "opus", DurationSeconds: 18}, *response.Audio)
				assert.Nil(t, response.Video)
			},
		},
		{
			name: "video with embedded audio",
			file: filedomain.ConfirmUploadResult{
				FileID:          "video-id",
				OriginalName:    "repair.mp4",
				MimeType:        "video/mp4",
				VideoCodec:      "h264",
				AudioCodec:      "aac",
				DurationSeconds: 24,
				Width:           1080,
				Height:          1920,
			},
			expectType: fileTypeVideo,
			assertBody: func(t *testing.T, response fileResponse) {
				require.NotNil(t, response.Video)
				assert.Equal(t, fileVideoResponse{
					VideoCodec:      "h264",
					AudioCodec:      "aac",
					DurationSeconds: 24,
					Width:           1080,
					Height:          1920,
				}, *response.Video)
				assert.Nil(t, response.Audio)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := fileResponseFromDomain(&test.file)
			assert.Equal(t, test.file.FileID, response.ID)
			assert.Equal(t, test.file.MimeType, response.MimeType)
			assert.Equal(t, test.expectType, response.Type)
			test.assertBody(t, response)
		})
	}
}

func TestFileResponseJSONKeepsMimeTypeCommonAndVariantMetadataNested(t *testing.T) {
	response := fileResponseFromDomain(&filedomain.ConfirmUploadResult{
		FileID:          "video-id",
		OriginalName:    "repair.mp4",
		MimeType:        "video/mp4",
		VideoCodec:      "h264",
		AudioCodec:      "aac",
		DurationSeconds: 24,
		Width:           1080,
		Height:          1920,
	})

	body, err := json.Marshal(response)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.Equal(t, "video/mp4", payload["mime_type"])
	assert.Equal(t, "video", payload["type"])
	assert.NotNil(t, payload["video"])
	assert.NotContains(t, payload, "codec")
	assert.NotContains(t, payload, "video_codec")
	assert.NotContains(t, payload, "audio_codec")
}
