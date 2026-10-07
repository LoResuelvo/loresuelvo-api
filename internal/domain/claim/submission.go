package claim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Reason string

const (
	ReasonNoncompliance  Reason = "non_compliance"
	ReasonImproperCharge Reason = "improper_charge"
	ReasonDamage         Reason = "damage"
	ReasonPoorQuality    Reason = "poor_quality"
)

func (r Reason) Valid() bool {
	return r == ReasonNoncompliance || r == ReasonImproperCharge || r == ReasonDamage || r == ReasonPoorQuality
}

type Submission struct {
	Reference    Reference
	Reason       Reason
	Description  string
	ImageFileIDs []string
}

func (s Submission) Normalize() (Submission, error) {
	ref, err := NewReference(s.Reference.Kind(), s.Reference.ID())
	if err != nil {
		return Submission{}, err
	}
	description := strings.TrimSpace(s.Description)
	if strings.ContainsRune(description, 0) || !s.Reason.Valid() || !utf8.ValidString(description) || utf8.RuneCountInString(description) < 1 || utf8.RuneCountInString(description) > 5000 || len(s.ImageFileIDs) > 3 {
		return Submission{}, ErrInvalidSubmission
	}
	images := make([]string, 0, len(s.ImageFileIDs))
	for _, id := range s.ImageFileIDs {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || len(id) != 36 {
			return Submission{}, ErrInvalidSubmission
		}
		images = append(images, parsed.String())
	}
	slices.Sort(images)
	for i := 1; i < len(images); i++ {
		if images[i] == images[i-1] {
			return Submission{}, ErrInvalidSubmission
		}
	}
	return Submission{Reference: ref, Reason: s.Reason, Description: description, ImageFileIDs: images}, nil
}
func (s Submission) Fingerprint() string {
	// An explicit positional encoding avoids ambiguous concatenation and ignores attachment order after normalization.
	bytes, err := json.Marshal([]any{s.Reference.Kind(), s.Reference.ID(), s.Reason, s.Description, s.ImageFileIDs})
	if err != nil {
		panic("claim fingerprint contains an unsupported value")
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}
func NormalizeIdempotencyKey(key string) (string, error) {
	parsed, err := uuid.Parse(key)
	if err != nil || parsed == uuid.Nil || len(key) != 36 {
		return "", ErrInvalidIdempotencyKey
	}
	return parsed.String(), nil
}
