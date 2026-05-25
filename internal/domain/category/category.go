package category

import "strings"

type Category struct {
	Name           string
	NormalizedName string
}

func New(name string) (*Category, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrNameRequired
	}

	return &Category{
		Name:           trimmedName,
		NormalizedName: strings.ToLower(trimmedName),
	}, nil
}
