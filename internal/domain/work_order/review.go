package workorder

import (
	"strings"
	"unicode/utf8"
)

const (
	minimumReviewRating          = 1
	maximumReviewRating          = 5
	maximumReviewDescriptionSize = 500
)

// Review is the consumer's immutable assessment of a paid work order.
// Ownership, identity, and persistence belong to the work order aggregate.
type Review struct {
	rating      int
	description string
}

func NewReview(rating int, description string) (*Review, error) {
	if rating < minimumReviewRating || rating > maximumReviewRating {
		return nil, ErrReviewRatingOutOfRange
	}

	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > maximumReviewDescriptionSize {
		return nil, ErrReviewDescriptionTooLong
	}

	return &Review{
		rating:      rating,
		description: description,
	}, nil
}

func (review *Review) Rating() int {
	if review == nil {
		return 0
	}
	return review.rating
}

func (review *Review) Description() string {
	if review == nil {
		return ""
	}
	return review.description
}
