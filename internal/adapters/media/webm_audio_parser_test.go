package media

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebMAudioParserParsesOpusMetadata(t *testing.T) {
	metadata, err := NewWebMAudioParser().Parse(testWebMOpusAudio(18))

	require.NoError(t, err)
	assert.Equal(t, filedomain.AudioMetadata{DurationSeconds: 18, Codec: "opus"}, metadata)
}

func TestWebMAudioParserRejectsUnsupportedObject(t *testing.T) {
	_, err := NewWebMAudioParser().Parse([]byte("not a WebM file"))

	assert.ErrorIs(t, err, filedomain.ErrUnsupportedMessageAudio)
}

func TestWebMAudioParserPreservesOversizedFiniteDurationForDomainPolicy(t *testing.T) {
	metadata, err := NewWebMAudioParser().Parse(testWebMOpusAudio(301))

	require.NoError(t, err)
	assert.Equal(t, 301, metadata.DurationSeconds)
}

func TestWebMAudioParserRejectsUnrepresentableDuration(t *testing.T) {
	_, err := NewWebMAudioParser().Parse(testWebMOpusAudio(math.MaxFloat64))

	assert.Error(t, err)
	assert.True(t, errors.Is(err, filedomain.ErrUnsupportedMessageAudio))
}

func testWebMOpusAudio(durationSeconds float64) []byte {
	duration := make([]byte, 8)
	binary.BigEndian.PutUint64(duration, math.Float64bits(durationSeconds*1000))

	ebmlHeader := testEBMLElement(0x1A45DFA3,
		testEBMLElement(0x4286, []byte{1}),
		testEBMLElement(0x4282, []byte("webm")),
	)
	info := testEBMLElement(0x1549A966,
		testEBMLElement(0x2AD7B1, []byte{0x0F, 0x42, 0x40}),
		testEBMLElement(0x4489, duration),
	)
	tracks := testEBMLElement(0x1654AE6B, testEBMLElement(0xAE, testEBMLElement(0x86, []byte("A_OPUS"))))
	segment := testEBMLElement(0x18538067, append(info, tracks...))
	return append(ebmlHeader, segment...)
}

func testEBMLElement(id uint64, payload ...[]byte) []byte {
	var body []byte
	for _, part := range payload {
		body = append(body, part...)
	}
	idBytes := testEBMLID(id)
	result := make([]byte, 0, len(idBytes)+8+len(body))
	result = append(result, idBytes...)
	result = append(result, testEBMLSize(len(body))...)
	result = append(result, body...)
	return result
}

func testEBMLID(id uint64) []byte {
	length := 1
	for value := id; value > 0xff; value >>= 8 {
		length++
	}
	result := make([]byte, length)
	for index := length - 1; index >= 0; index-- {
		result[index] = byte(id)
		id >>= 8
	}
	return result
}

func testEBMLSize(size int) []byte {
	for length := 1; length <= 8; length++ {
		max := (uint64(1) << uint(7*length)) - 2
		if uint64(size) > max {
			continue
		}
		result := make([]byte, length)
		value := uint64(size)
		for index := length - 1; index >= 0; index-- {
			result[index] = byte(value)
			value >>= 8
		}
		result[0] |= byte(1 << uint(8-length))
		return result
	}
	panic("test WebM element payload is too large")
}
