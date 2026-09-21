package category

import (
	"strings"
	"unicode/utf8"
)

const maximumNameLength = 100

type Category struct {
	ID             int
	Name           string
	NormalizedName string
}

func New(name string) (*Category, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrNameRequired
	}
	if utf8.RuneCountInString(trimmedName) > maximumNameLength {
		return nil, ErrNameTooLong
	}

	return &Category{
		Name:           trimmedName,
		NormalizedName: strings.ToLower(trimmedName),
	}, nil
}
