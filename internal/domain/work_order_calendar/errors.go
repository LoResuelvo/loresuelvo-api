package workordercalendar

import "errors"

var (
	ErrCalendarEventNotFound      = errors.New("calendar event does not exist")
	ErrCalendarEventIdentity      = errors.New("calendar event identity is required")
	ErrCalendarEventCalendarID    = errors.New("calendar event calendar id is required")
	ErrCalendarEventExternalID    = errors.New("calendar event external id is required")
	ErrCalendarEventAlreadySynced = errors.New("calendar event is already synchronized")
	ErrCalendarEventSyncedOn      = errors.New("calendar event synchronized timestamp is required")
)
