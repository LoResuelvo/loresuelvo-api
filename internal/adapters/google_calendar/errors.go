package googlecalendar

import "errors"

var (
	ErrInvalidOAuthConfiguration     = errors.New("Google Calendar OAuth configuration is incomplete")
	ErrAuthorizationCodeUnusable     = errors.New("Google Calendar authorization code cannot be used")
	ErrAuthorizationGrantUnavailable = errors.New("Google Calendar authorization grant is unavailable")
)
