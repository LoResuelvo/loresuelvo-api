package calendarconnection

import (
	"strings"
	"time"
)

const (
	StatusConnected      = "connected"
	StatusActionRequired = "action_required"
	StatusDisconnected   = "disconnected"
)

type Connection struct {
	userID                 int
	refreshTokenCiphertext []byte
	calendarID             string
	status                 string
	connectedOn            time.Time
	updatedOn              time.Time
}

func NewConnection(userID int, calendarID string, refreshTokenCiphertext []byte, connectedOn time.Time) (*Connection, error) {
	if userID <= 0 {
		return nil, ErrUserIDRequired
	}
	calendarID = strings.TrimSpace(calendarID)
	if calendarID == "" {
		return nil, ErrCalendarIDRequired
	}
	if len(refreshTokenCiphertext) == 0 {
		return nil, ErrRefreshTokenRequired
	}
	connectedOn = connectedOn.UTC()
	return &Connection{
		userID:                 userID,
		refreshTokenCiphertext: append([]byte(nil), refreshTokenCiphertext...),
		calendarID:             calendarID,
		status:                 StatusConnected,
		connectedOn:            connectedOn,
		updatedOn:              connectedOn,
	}, nil
}

func RehydrateConnection(userID int, refreshTokenCiphertext []byte, calendarID, status string, connectedOn, updatedOn time.Time) (*Connection, error) {
	if userID <= 0 {
		return nil, ErrUserIDRequired
	}
	if len(refreshTokenCiphertext) == 0 {
		return nil, ErrRefreshTokenRequired
	}
	calendarID = strings.TrimSpace(calendarID)
	if calendarID == "" {
		return nil, ErrCalendarIDRequired
	}
	status = strings.TrimSpace(status)
	if status != StatusConnected && status != StatusActionRequired {
		return nil, ErrConnectionStatusInvalid
	}
	return &Connection{
		userID:                 userID,
		refreshTokenCiphertext: append([]byte(nil), refreshTokenCiphertext...),
		calendarID:             calendarID,
		status:                 status,
		connectedOn:            connectedOn.UTC(),
		updatedOn:              updatedOn.UTC(),
	}, nil
}

func (connection *Connection) UserID() int { return connection.userID }

func (connection *Connection) RefreshTokenCiphertext() []byte {
	return append([]byte(nil), connection.refreshTokenCiphertext...)
}

func (connection *Connection) CalendarID() string { return connection.calendarID }

func (connection *Connection) Status() string { return connection.status }

func (connection *Connection) IsConnected() bool {
	return connection.status == StatusConnected
}

func (connection *Connection) ConnectedOn() time.Time { return connection.connectedOn }

func (connection *Connection) UpdatedOn() time.Time { return connection.updatedOn }

func (connection *Connection) MarkActionRequired(now time.Time) {
	connection.status = StatusActionRequired
	connection.updatedOn = now.UTC()
}
