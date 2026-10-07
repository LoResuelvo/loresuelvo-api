package installation

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/google/uuid"
)

type Installation struct {
	ID         string
	UserID     int
	App        string
	Token      string
	Locale     string
	BindingID  string
	Enabled    bool
	SecretHash []byte
	Revision   int64
}

func New(id string, userID int, app, token, locale, bindingID string) (*Installation, error) {
	id, token, bindingID = strings.TrimSpace(id), strings.TrimSpace(token), strings.TrimSpace(bindingID)
	if id == "" || len(id) > 200 || userID <= 0 || (app != "consumer" && app != "provider") || token == "" || len(token) > 4096 || bindingID == "" || len(bindingID) > 200 || (locale != "" && locale != "es" && locale != "en") {
		return nil, ErrInvalidInstallation
	}
	if locale == "" {
		locale = "es"
	}
	return &Installation{ID: id, UserID: userID, App: app, Token: token, Locale: locale, BindingID: bindingID, Enabled: true}, nil
}

// Registration represents the device's current login and possession proof.
type Registration struct {
	ID                string
	Secret            string
	App               string
	Token             string
	Locale            string
	BindingID         string
	PreviousBindingID string
}

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.Version() == 4 && parsed.Variant() == uuid.RFC4122 && parsed.String() == value
}
func (r Registration) Validate() error {
	if !validUUID(r.ID) || !validUUID(r.Secret) || !validUUID(r.BindingID) || (r.PreviousBindingID != "" && !validUUID(r.PreviousBindingID)) {
		return ErrInvalidInstallation
	}
	if strings.TrimSpace(r.Token) != r.Token || strings.ContainsAny(r.Token, " \t\r\n") {
		return ErrInvalidInstallation
	}
	_, err := New(r.ID, 1, r.App, r.Token, r.Locale, r.BindingID)
	return err
}
func NewRegistered(userID int, role string, r Registration) (*Installation, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if role != r.App {
		return nil, ErrForbidden
	}
	if r.PreviousBindingID != "" {
		return nil, ErrConflict
	}
	i, err := New(r.ID, userID, r.App, r.Token, r.Locale, r.BindingID)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(r.Secret))
	i.SecretHash = hash[:]
	return i, nil
}
func (i *Installation) provesPossession(secret string) bool {
	hash := sha256.Sum256([]byte(secret))
	return len(i.SecretHash) == sha256.Size && subtle.ConstantTimeCompare(hash[:], i.SecretHash) == 1
}
func (i *Installation) Register(userID int, role string, r Registration) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if i.ID != r.ID || role != r.App || i.App != r.App || !i.provesPossession(r.Secret) {
		return ErrForbidden
	}
	if i.BindingID == r.BindingID {
		if i.UserID != userID {
			return ErrForbidden
		}
		if !i.Enabled {
			return ErrConflict
		}
	} else if r.PreviousBindingID != i.BindingID {
		return ErrConflict
	}
	renewed, err := New(r.ID, userID, r.App, r.Token, r.Locale, r.BindingID)
	if err != nil {
		return err
	}
	i.UserID, i.Token, i.Locale, i.BindingID, i.Enabled = renewed.UserID, renewed.Token, renewed.Locale, renewed.BindingID, true
	return nil
}
func (i *Installation) Unregister(userID int, secret, bindingID string) error {
	if !validUUID(secret) || !validUUID(bindingID) {
		return ErrInvalidInstallation
	}
	if i.UserID != userID || !i.provesPossession(secret) {
		return ErrForbidden
	}
	if i.BindingID != bindingID {
		return ErrConflict
	}
	i.Enabled = false
	return nil
}
