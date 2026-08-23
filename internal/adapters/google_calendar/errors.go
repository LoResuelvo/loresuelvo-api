package googlecalendar

import "errors"

var (
	ErrInvalidOAuthConfiguration     = errors.New("Google Calendar OAuth configuration is incomplete")
	ErrAuthorizationCodeUnusable     = errors.New("Google Calendar authorization code cannot be used")
	ErrAuthorizationGrantUnavailable = errors.New("Google Calendar authorization grant is unavailable")
	ErrCalendarEventTokenUnavailable = errors.New("Google Calendar access token is unavailable")
	ErrCalendarEventCreationFailed   = errors.New("Google Calendar event could not be created")
)
