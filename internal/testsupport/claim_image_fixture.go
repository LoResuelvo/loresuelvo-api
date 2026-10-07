package testsupport

import (
	"bytes"
	"image"
	"image/png"
)

// ClaimImagePNG returns real encoded bytes, rather than fabricating confirmed metadata.
func ClaimImagePNG() ([]byte, error) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
