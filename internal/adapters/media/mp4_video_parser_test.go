package media

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMP4VideoParserParsesVideoWithoutAudio(t *testing.T) {
	metadata, err := NewMP4VideoParser().Parse(testMP4Video(18, false))

	require.NoError(t, err)
	require.Equal(t, 18, metadata.DurationSeconds)
	require.Equal(t, "h264", metadata.VideoCodec)
	require.Empty(t, metadata.AudioCodec)
	require.Equal(t, 1080, metadata.Width)
	require.Equal(t, 1920, metadata.Height)
}

func TestMP4VideoParserParsesOptionalAACAudio(t *testing.T) {
	metadata, err := NewMP4VideoParser().Parse(testMP4Video(24, true))

	require.NoError(t, err)
	require.Equal(t, 24, metadata.DurationSeconds)
	require.Equal(t, "h264", metadata.VideoCodec)
	require.Equal(t, "aac", metadata.AudioCodec)
}

func TestMP4VideoParserRejectsTruncatedObject(t *testing.T) {
	_, err := NewMP4VideoParser().Parse([]byte("not-an-mp4"))

	require.Error(t, err)
}

func testMP4Video(durationSeconds int, withAudio bool) []byte {
	mvhd := make([]byte, 20)
	binary.BigEndian.PutUint32(mvhd[12:16], 1000)
	binary.BigEndian.PutUint32(mvhd[16:20], uint32(durationSeconds*1000))

	videoTkhd := make([]byte, 84)
	binary.BigEndian.PutUint32(videoTkhd[76:80], uint32(1080)<<16)
	binary.BigEndian.PutUint32(videoTkhd[80:84], uint32(1920)<<16)
	videoStsd := make([]byte, 8)
	videoEntry := make([]byte, 28)
	binary.BigEndian.PutUint16(videoEntry[24:26], 1080)
	binary.BigEndian.PutUint16(videoEntry[26:28], 1920)
	videoStsd = append(videoStsd, mp4TestBox("avc1", videoEntry)...)
	videoTrack := mp4TestBox("trak",
		mp4TestBox("tkhd", videoTkhd),
		mp4TestBox("mdia",
			mp4TestBox("hdlr", append(make([]byte, 8), []byte("vide")...)),
			mp4TestBox("minf", mp4TestBox("stbl", mp4TestBox("stsd", videoStsd))),
		),
	)

	moovPayload := append(mp4TestBox("mvhd", mvhd), videoTrack...)
	if withAudio {
		audioStsd := append(make([]byte, 8), mp4TestBox("mp4a")...)
		audioTrack := mp4TestBox("trak",
			mp4TestBox("mdia",
				mp4TestBox("hdlr", append(make([]byte, 8), []byte("soun")...)),
				mp4TestBox("minf", mp4TestBox("stbl", mp4TestBox("stsd", audioStsd))),
			),
		)
		moovPayload = append(moovPayload, audioTrack...)
	}

	ftyp := append([]byte("isom"), make([]byte, 4)...)
	ftyp = append(ftyp, []byte("isommp42")...)
	return append(
		mp4TestBox("ftyp", ftyp),
		append(mp4TestBox("moov", moovPayload), mp4TestBox("mdat", []byte{0})...)...,
	)
}

func mp4TestBox(typ string, payload ...[]byte) []byte {
	body := make([]byte, 0)
	for _, part := range payload {
		body = append(body, part...)
	}
	result := make([]byte, 8+len(body))
	binary.BigEndian.PutUint32(result[:4], uint32(len(result)))
	copy(result[4:8], typ)
	copy(result[8:], body)
	return result
}
