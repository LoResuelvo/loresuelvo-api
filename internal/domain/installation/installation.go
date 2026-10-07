package installation

import (
	"errors"
	"strings"
)

var ErrInvalidInstallation = errors.New("invalid installation")

type Installation struct {
	ID        string
	UserID    int
	App       string
	Token     string
	Locale    string
	BindingID string
	Enabled   bool
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
