package evals

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const maxSemanticReviewBytes = 16 * 1024 * 1024

func ReadSemanticReviews(path string) (SemanticReviewDocument, error) {
	var document SemanticReviewDocument
	file, err := os.Open(path)
	if err != nil {
		return document, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxSemanticReviewBytes+1))
	if err = errors.Join(readErr, file.Close()); err != nil {
		return document, err
	}
	if len(data) > maxSemanticReviewBytes {
		return document, fmt.Errorf("%w: review document exceeds size limit", ErrInvalidSemanticReview)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&document); err != nil {
		return document, fmt.Errorf("decode semantic reviews: %w", err)
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return document, fmt.Errorf("%w: trailing review data", ErrInvalidSemanticReview)
	}
	return document, nil
}

// WriteSemanticReviews never overwrites a prior review. Revisions use a new
// explicit filename; imported judgments do not alter the original run journal.
func WriteSemanticReviews(path string, document SemanticReviewDocument) (resultErr error) {
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode semantic reviews: %w", err)
	}
	if len(data)+1 > maxSemanticReviewBytes {
		return fmt.Errorf("%w: review document exceeds size limit", ErrInvalidSemanticReview)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if _, err = file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}
