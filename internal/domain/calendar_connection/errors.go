package calendarconnection

import "errors"

var (
	ErrAuthorizationStateRequired   = errors.New("calendar authorization state is required")
	ErrAuthorizationCodeRequired    = errors.New("calendar authorization code is required")
	ErrAuthorizationAttemptNotFound = errors.New("calendar authorization attempt does not exist")
	ErrAuthorizationAttemptExpired  = errors.New("calendar authorization attempt has expired")
	ErrAuthorizationAttemptConsumed = errors.New("calendar authorization attempt has already been consumed")
	ErrRefreshTokenRequired         = errors.New("calendar refresh token is required")
	ErrCalendarIDRequired           = errors.New("calendar id is required")
	ErrConnectionNotFound           = errors.New("calendar connection does not exist")
	ErrUserNotFound                 = errors.New("calendar connection user does not exist")
	ErrUserIDRequired               = errors.New("calendar connection user id is required")
	ErrConnectionStatusInvalid      = errors.New("calendar connection status is invalid")
	ErrConnectionWriterUnavailable  = errors.New("calendar connection writer is unavailable")
)
