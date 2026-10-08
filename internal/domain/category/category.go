package category

import (
	"strings"
	"unicode/utf8"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
)

const maximumNameLength = 100

type Category struct {
	ID             int
	Name           string
	NormalizedName string
	Enabled        bool
	Version        int
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
		Enabled:        true,
		Version:        1,
	}, nil
}

// Edit carries only the mutable business fields and the expected concurrency token.
type Edit struct {
	ExpectedVersion      int
	Name                 *string
	Enabled              *bool
	Reason               string
	ConfirmOngoingOrders bool
}

func (edit Edit) Validate() error {
	if edit.ExpectedVersion <= 0 {
		return ErrVersionRequired
	}
	if edit.Name == nil && edit.Enabled == nil {
		return ErrNoChanges
	}
	if edit.Name != nil {
		if _, err := New(*edit.Name); err != nil {
			return err
		}
	}
	if edit.Reason != "" {
		if _, err := audit.NewReason(edit.Reason); err != nil {
			return ErrInvalidReason
		}
	}
	return nil
}

// Edit validates the complete transition before replacing any current value.
func (category *Category) Edit(edit Edit, hasOngoingOrders bool) (bool, error) {
	if err := edit.Validate(); err != nil {
		return false, err
	}
	if category.Version != edit.ExpectedVersion {
		return false, ErrVersionConflict
	}
	next := *category
	if edit.Name != nil {
		named, err := New(*edit.Name)
		if err != nil {
			return false, err
		}
		next.Name, next.NormalizedName = named.Name, named.NormalizedName
	}
	if edit.Enabled != nil {
		next.Enabled = *edit.Enabled
	}
	if next.Enabled != category.Enabled {
		if _, err := audit.NewReason(edit.Reason); err != nil {
			return false, ErrInvalidReason
		}
		if !next.Enabled && hasOngoingOrders && !edit.ConfirmOngoingOrders {
			return false, ErrConfirmationRequired
		}
	}
	if next.Name == category.Name && next.Enabled == category.Enabled {
		return false, nil
	}
	next.Version++
	*category = next
	return true, nil
}

func (category Category) RequireEnabled() error {
	if !category.Enabled {
		return ErrDisabled
	}
	return nil
}
