package media

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

const (
	ebmlElementID        uint64 = 0x1A45DFA3
	docTypeElementID     uint64 = 0x4282
	segmentElementID     uint64 = 0x18538067
	infoElementID        uint64 = 0x1549A966
	timecodeScaleElement uint64 = 0x2AD7B1
	durationElementID    uint64 = 0x4489
	tracksElementID      uint64 = 0x1654AE6B
	trackEntryElementID  uint64 = 0xAE
	codecIDElementID     uint64 = 0x86
)

const defaultWebMTimecodeScale = uint64(1_000_000)

type WebMAudioParser struct{}

func NewWebMAudioParser() *WebMAudioParser {
	return &WebMAudioParser{}
}

func (WebMAudioParser) Parse(data []byte) (filedomain.AudioMetadata, error) {
	return parseWebMAudio(data)
}

func parseWebMAudio(data []byte) (filedomain.AudioMetadata, error) {
	if len(data) == 0 {
		return filedomain.AudioMetadata{}, unsupportedAudio("empty object")
	}

	ebmlElement, err := readEBMLElement(data, 0, len(data))
	if err != nil || ebmlElement.id != ebmlElementID {
		return filedomain.AudioMetadata{}, unsupportedAudio("missing EBML header")
	}
	docType, err := readDocType(data, ebmlElement.payloadStart, ebmlElement.payloadEnd)
	if err != nil || docType != "webm" {
		return filedomain.AudioMetadata{}, unsupportedAudio("object is not a WebM document")
	}

	segmentElement, err := readEBMLElement(data, ebmlElement.payloadEnd, len(data))
	if err != nil || segmentElement.id != segmentElementID {
		return filedomain.AudioMetadata{}, unsupportedAudio("missing WebM segment")
	}

	metadata := filedomain.AudioMetadata{}
	timecodeScale := defaultWebMTimecodeScale
	if err := scanSegment(data, segmentElement.payloadStart, segmentElement.payloadEnd, &metadata, &timecodeScale); err != nil {
		return filedomain.AudioMetadata{}, err
	}
	if metadata.Codec != "A_OPUS" {
		return filedomain.AudioMetadata{}, unsupportedAudio("WebM track is not Opus")
	}
	if metadata.DurationSeconds <= 0 {
		return filedomain.AudioMetadata{}, unsupportedAudio("WebM duration is missing or invalid")
	}

	metadata.Codec = "opus"
	return metadata, nil
}

func readDocType(data []byte, start, end int) (string, error) {
	for offset := start; offset < end; {
		element, err := readEBMLElement(data, offset, end)
		if err != nil {
			return "", err
		}
		if element.id == docTypeElementID {
			return strings.TrimSpace(string(data[element.payloadStart:element.payloadEnd])), nil
		}
		offset = element.payloadEnd
	}

	return "", unsupportedAudio("WebM document type is missing")
}

func scanSegment(data []byte, start, end int, metadata *filedomain.AudioMetadata, timecodeScale *uint64) error {
	for offset := start; offset < end; {
		element, err := readEBMLElement(data, offset, end)
		if err != nil {
			return err
		}

		switch element.id {
		case infoElementID:
			if err := scanInfo(data, element.payloadStart, element.payloadEnd, metadata, timecodeScale); err != nil {
				return err
			}
		case tracksElementID:
			if err := scanTracks(data, element.payloadStart, element.payloadEnd, metadata); err != nil {
				return err
			}
		}
		offset = element.payloadEnd
	}

	return nil
}

func scanInfo(data []byte, start, end int, metadata *filedomain.AudioMetadata, timecodeScale *uint64) error {
	var duration float64
	for offset := start; offset < end; {
		element, err := readEBMLElement(data, offset, end)
		if err != nil {
			return err
		}

		switch element.id {
		case timecodeScaleElement:
			scale, err := readUnsignedInteger(data[element.payloadStart:element.payloadEnd])
			if err != nil || scale == 0 {
				return unsupportedAudio("invalid WebM timecode scale")
			}
			*timecodeScale = scale
		case durationElementID:
			value, err := readEBMLFloat(data[element.payloadStart:element.payloadEnd])
			if err != nil {
				return unsupportedAudio("invalid WebM duration")
			}
			duration = value
		}
		offset = element.payloadEnd
	}

	if duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) {
		seconds := duration * float64(*timecodeScale) / 1_000_000_000
		if seconds > 0 && !math.IsNaN(seconds) && !math.IsInf(seconds, 0) {
			roundedSeconds := math.Ceil(seconds)
			maxInt := int(^uint(0) >> 1)
			if roundedSeconds >= float64(maxInt) {
				metadata.DurationSeconds = maxInt
				return nil
			}
			metadata.DurationSeconds = int(roundedSeconds)
		}
	}

	return nil
}

func scanTracks(data []byte, start, end int, metadata *filedomain.AudioMetadata) error {
	for offset := start; offset < end; {
		element, err := readEBMLElement(data, offset, end)
		if err != nil {
			return err
		}
		if element.id == trackEntryElementID {
			codec, err := readTrackCodec(data, element.payloadStart, element.payloadEnd)
			if err != nil {
				return err
			}
			if codec == "A_OPUS" {
				metadata.Codec = codec
			}
		}
		offset = element.payloadEnd
	}

	return nil
}

func readTrackCodec(data []byte, start, end int) (string, error) {
	for offset := start; offset < end; {
		element, err := readEBMLElement(data, offset, end)
		if err != nil {
			return "", err
		}
		if element.id == codecIDElementID {
			return strings.TrimSpace(string(data[element.payloadStart:element.payloadEnd])), nil
		}
		offset = element.payloadEnd
	}

	return "", nil
}

type ebmlElement struct {
	id           uint64
	payloadStart int
	payloadEnd   int
}

func readEBMLElement(data []byte, offset, end int) (ebmlElement, error) {
	id, idLength, err := readElementID(data, offset, end)
	if err != nil {
		return ebmlElement{}, err
	}
	size, sizeLength, unknownSize, err := readElementSize(data, offset+idLength, end)
	if err != nil {
		return ebmlElement{}, err
	}
	payloadStart := offset + idLength + sizeLength
	if payloadStart > end {
		return ebmlElement{}, fmt.Errorf("%w: element header exceeds object", filedomain.ErrUnsupportedMessageAudio)
	}
	payloadEnd := end
	if !unknownSize {
		if size > uint64(end-payloadStart) {
			return ebmlElement{}, fmt.Errorf("%w: element payload exceeds parent", filedomain.ErrUnsupportedMessageAudio)
		}
		payloadEnd = payloadStart + int(size)
	}

	return ebmlElement{id: id, payloadStart: payloadStart, payloadEnd: payloadEnd}, nil
}

func readElementID(data []byte, offset, end int) (uint64, int, error) {
	if offset >= end {
		return 0, 0, fmt.Errorf("%w: missing element id", filedomain.ErrUnsupportedMessageAudio)
	}
	length := vintLength(data[offset])
	if length == 0 || length > 4 || offset+length > end {
		return 0, 0, fmt.Errorf("%w: invalid element id", filedomain.ErrUnsupportedMessageAudio)
	}

	var id uint64
	for index := 0; index < length; index++ {
		id = id<<8 | uint64(data[offset+index])
	}
	return id, length, nil
}

func readElementSize(data []byte, offset, end int) (uint64, int, bool, error) {
	if offset >= end {
		return 0, 0, false, fmt.Errorf("%w: missing element size", filedomain.ErrUnsupportedMessageAudio)
	}
	length := vintLength(data[offset])
	if length == 0 || length > 8 || offset+length > end {
		return 0, 0, false, fmt.Errorf("%w: invalid element size", filedomain.ErrUnsupportedMessageAudio)
	}

	value := uint64(data[offset]) & (uint64(0xFF) >> uint(length))
	for index := 1; index < length; index++ {
		value = value<<8 | uint64(data[offset+index])
	}
	unknownSize := value == (uint64(1)<<uint(7*length))-1
	return value, length, unknownSize, nil
}

func vintLength(firstByte byte) int {
	for length, bit := 1, byte(0x80); bit != 0; length, bit = length+1, bit>>1 {
		if firstByte&bit != 0 {
			return length
		}
	}
	return 0
}

func readUnsignedInteger(data []byte) (uint64, error) {
	if len(data) == 0 || len(data) > 8 {
		return 0, fmt.Errorf("%w: invalid unsigned integer", filedomain.ErrUnsupportedMessageAudio)
	}
	var value uint64
	for _, part := range data {
		value = value<<8 | uint64(part)
	}
	return value, nil
}

func readEBMLFloat(data []byte) (float64, error) {
	switch len(data) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(data))), nil
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(data)), nil
	default:
		return 0, fmt.Errorf("%w: duration must be a 32-bit or 64-bit float", filedomain.ErrUnsupportedMessageAudio)
	}
}

func unsupportedAudio(reason string) error {
	return fmt.Errorf("%s: %w", reason, filedomain.ErrUnsupportedMessageAudio)
}
