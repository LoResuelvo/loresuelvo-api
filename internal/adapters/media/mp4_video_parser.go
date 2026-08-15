package media

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

// MP4VideoParser extracts the small, security-relevant subset of ISO-BMFF metadata
// required by conversation videos without invoking an external binary.
type MP4VideoParser struct{}

func NewMP4VideoParser() *MP4VideoParser {
	return &MP4VideoParser{}
}

func (MP4VideoParser) Parse(data []byte) (filedomain.VideoMetadata, error) {
	return parseMP4Video(data)
}

type mp4Box struct {
	typ     string
	payload []byte
}

func parseMP4Video(data []byte) (filedomain.VideoMetadata, error) {
	boxes, err := readMP4Boxes(data)
	if err != nil {
		return filedomain.VideoMetadata{}, unsupportedVideo(err.Error())
	}

	var moov *mp4Box
	hasFileType := false
	hasMediaData := false
	for index := range boxes {
		switch boxes[index].typ {
		case "ftyp":
			hasFileType = validMP4FileType(boxes[index].payload)
		case "moov":
			moov = &boxes[index]
		case "mdat":
			hasMediaData = len(boxes[index].payload) > 0
		}
	}
	if !hasFileType || moov == nil || !hasMediaData {
		return filedomain.VideoMetadata{}, unsupportedVideo("missing MP4 structure")
	}

	movieBoxes, err := readMP4Boxes(moov.payload)
	if err != nil {
		return filedomain.VideoMetadata{}, unsupportedVideo("invalid MP4 movie box")
	}
	timescale, duration, err := movieDuration(movieBoxes)
	if err != nil {
		return filedomain.VideoMetadata{}, unsupportedVideo(err.Error())
	}

	metadata := filedomain.VideoMetadata{}
	for _, movieBox := range movieBoxes {
		if movieBox.typ != "trak" {
			continue
		}
		trackMetadata, err := parseMP4Track(movieBox.payload)
		if err != nil {
			return filedomain.VideoMetadata{}, unsupportedVideo(err.Error())
		}
		switch trackMetadata.handlerType {
		case "vide":
			if metadata.VideoCodec != "" {
				continue
			}
			metadata.VideoCodec = trackMetadata.codec
			metadata.Width = trackMetadata.width
			metadata.Height = trackMetadata.height
		case "soun":
			if metadata.AudioCodec == "" {
				metadata.AudioCodec = trackMetadata.codec
			}
		}
	}

	if metadata.VideoCodec == "" || metadata.Width <= 0 || metadata.Height <= 0 {
		return filedomain.VideoMetadata{}, unsupportedVideo("missing H.264 video track")
	}
	if timescale == 0 || duration == 0 {
		return filedomain.VideoMetadata{}, unsupportedVideo("missing MP4 duration")
	}
	metadata.DurationSeconds = ceilDurationSeconds(duration, timescale)
	return metadata, nil
}

type mp4TrackMetadata struct {
	handlerType string
	codec       string
	width       int
	height      int
}

func parseMP4Track(data []byte) (mp4TrackMetadata, error) {
	trackBoxes, err := readMP4Boxes(data)
	if err != nil {
		return mp4TrackMetadata{}, fmt.Errorf("invalid MP4 track box")
	}

	var tkhd, mdia *mp4Box
	for index := range trackBoxes {
		switch trackBoxes[index].typ {
		case "tkhd":
			tkhd = &trackBoxes[index]
		case "mdia":
			mdia = &trackBoxes[index]
		}
	}
	if mdia == nil {
		return mp4TrackMetadata{}, fmt.Errorf("missing MP4 media box")
	}

	mediaBoxes, err := readMP4Boxes(mdia.payload)
	if err != nil {
		return mp4TrackMetadata{}, fmt.Errorf("invalid MP4 media box")
	}
	var handlerType string
	var minf *mp4Box
	for index := range mediaBoxes {
		switch mediaBoxes[index].typ {
		case "hdlr":
			handlerType, err = parseHandlerType(mediaBoxes[index].payload)
		case "minf":
			minf = &mediaBoxes[index]
		}
		if err != nil {
			return mp4TrackMetadata{}, err
		}
	}
	if handlerType == "" || minf == nil {
		return mp4TrackMetadata{}, fmt.Errorf("missing MP4 track metadata")
	}

	minfBoxes, err := readMP4Boxes(minf.payload)
	if err != nil {
		return mp4TrackMetadata{}, fmt.Errorf("invalid MP4 media information box")
	}
	var stbl *mp4Box
	for index := range minfBoxes {
		if minfBoxes[index].typ == "stbl" {
			stbl = &minfBoxes[index]
			break
		}
	}
	if stbl == nil {
		return mp4TrackMetadata{}, fmt.Errorf("missing MP4 sample table")
	}

	stblBoxes, err := readMP4Boxes(stbl.payload)
	if err != nil {
		return mp4TrackMetadata{}, fmt.Errorf("invalid MP4 sample table")
	}
	var stsd *mp4Box
	for index := range stblBoxes {
		if stblBoxes[index].typ == "stsd" {
			stsd = &stblBoxes[index]
			break
		}
	}
	if stsd == nil {
		return mp4TrackMetadata{}, fmt.Errorf("missing MP4 sample description")
	}
	codec, width, height, err := parseSampleDescription(stsd.payload, handlerType)
	if err != nil {
		return mp4TrackMetadata{}, err
	}
	if handlerType == "vide" && (width <= 0 || height <= 0) && tkhd != nil {
		width, height = parseTrackDimensions(tkhd.payload)
	}
	return mp4TrackMetadata{handlerType: handlerType, codec: codec, width: width, height: height}, nil
}

func movieDuration(boxes []mp4Box) (uint32, uint64, error) {
	for _, box := range boxes {
		if box.typ != "mvhd" {
			continue
		}
		if len(box.payload) < 20 {
			return 0, 0, fmt.Errorf("invalid MP4 movie header")
		}
		version := box.payload[0]
		if version == 0 {
			if len(box.payload) < 20 {
				return 0, 0, fmt.Errorf("invalid MP4 movie header")
			}
			return binary.BigEndian.Uint32(box.payload[12:16]), uint64(binary.BigEndian.Uint32(box.payload[16:20])), nil
		}
		if version == 1 {
			if len(box.payload) < 32 {
				return 0, 0, fmt.Errorf("invalid MP4 movie header")
			}
			return binary.BigEndian.Uint32(box.payload[20:24]), binary.BigEndian.Uint64(box.payload[24:32]), nil
		}
		return 0, 0, fmt.Errorf("unsupported MP4 movie header version")
	}
	return 0, 0, fmt.Errorf("missing MP4 movie header")
}

func parseHandlerType(data []byte) (string, error) {
	if len(data) < 12 {
		return "", fmt.Errorf("invalid MP4 handler")
	}
	return string(data[8:12]), nil
}

func parseSampleDescription(data []byte, handlerType string) (string, int, int, error) {
	if len(data) < 8 {
		return "", 0, 0, fmt.Errorf("invalid MP4 sample description")
	}
	entries, err := readMP4Boxes(data[8:])
	if err != nil || len(entries) == 0 {
		return "", 0, 0, fmt.Errorf("missing MP4 sample entry")
	}
	entry := entries[0]
	codec := normalizeMP4Codec(entry.typ, handlerType)
	if codec == "" {
		return "", 0, 0, fmt.Errorf("missing MP4 codec")
	}
	if handlerType != "vide" {
		return codec, 0, 0, nil
	}
	// A VisualSampleEntry stores width and height after the six reserved
	// bytes, data-reference index, and the three pre-defined 32-bit values.
	// Some files omit the optional sample-entry fields we do not need; in
	// that case the track header remains a valid dimensions fallback.
	if len(entry.payload) < 28 {
		return codec, 0, 0, nil
	}
	return codec, int(binary.BigEndian.Uint16(entry.payload[24:26])), int(binary.BigEndian.Uint16(entry.payload[26:28])), nil
}

func normalizeMP4Codec(sampleEntryType, handlerType string) string {
	switch {
	case handlerType == "vide" && (sampleEntryType == "avc1" || sampleEntryType == "avc3"):
		return "h264"
	case handlerType == "soun" && sampleEntryType == "mp4a":
		return "aac"
	default:
		return strings.ToLower(strings.TrimSpace(sampleEntryType))
	}
}

func parseTrackDimensions(data []byte) (int, int) {
	if len(data) < 84 {
		return 0, 0
	}
	version := data[0]
	offset := 76
	if version == 1 {
		offset = 88
	}
	if len(data) < offset+8 {
		return 0, 0
	}
	return int(binary.BigEndian.Uint32(data[offset:offset+4]) >> 16), int(binary.BigEndian.Uint32(data[offset+4:offset+8]) >> 16)
}

func validMP4FileType(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	majorBrand := string(data[:4])
	if majorBrand == "isom" || majorBrand == "iso2" || majorBrand == "mp41" || majorBrand == "mp42" || majorBrand == "avc1" {
		return true
	}
	for offset := 8; offset+4 <= len(data); offset += 4 {
		if string(data[offset:offset+4]) == "isom" || string(data[offset:offset+4]) == "mp42" {
			return true
		}
	}
	return false
}

func readMP4Boxes(data []byte) ([]mp4Box, error) {
	boxes := make([]mp4Box, 0)
	for offset := 0; offset < len(data); {
		if len(data)-offset < 8 {
			return nil, fmt.Errorf("truncated MP4 box header")
		}
		size := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		headerSize := uint64(8)
		if size == 1 {
			if len(data)-offset < 16 {
				return nil, fmt.Errorf("truncated MP4 large box header")
			}
			size = binary.BigEndian.Uint64(data[offset+8 : offset+16])
			headerSize = 16
		} else if size == 0 {
			size = uint64(len(data) - offset)
		}
		if size < headerSize || size > uint64(len(data)-offset) {
			return nil, fmt.Errorf("invalid MP4 box size")
		}
		boxEnd := offset + int(size)
		boxes = append(boxes, mp4Box{typ: string(data[offset+4 : offset+8]), payload: data[offset+int(headerSize) : boxEnd]})
		offset = boxEnd
	}
	return boxes, nil
}

func ceilDurationSeconds(duration uint64, timescale uint32) int {
	seconds := (duration + uint64(timescale) - 1) / uint64(timescale)
	maxInt := uint64(^uint(0) >> 1)
	if seconds >= maxInt || seconds > uint64(math.MaxInt) {
		return int(maxInt)
	}
	return int(seconds)
}

func unsupportedVideo(reason string) error {
	return fmt.Errorf("%w: %s", filedomain.ErrUnsupportedMessageVideo, reason)
}
