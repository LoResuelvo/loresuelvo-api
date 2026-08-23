package workordercalendar

import "errors"

var (
	ErrCalendarEventIdentity   = errors.New("calendar event identity is required")
	ErrCalendarEventCalendarID = errors.New("calendar event calendar id is required")
	ErrCalendarEventExternalID = errors.New("calendar event external id is required")
	ErrCalendarEventSyncedOn   = errors.New("calendar event synchronized timestamp is required")
)
