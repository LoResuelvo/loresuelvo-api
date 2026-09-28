package admin_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"net/url"
	"strconv"
	"time"
)

const consumerHistoryCursorPurpose = "admin-consumer-history:v1"

type consumerHistoryCursor struct {
	Version    int                               `json:"v"`
	ConsumerID int                               `json:"consumer_id"`
	Type       string                            `json:"type"`
	Status     string                            `json:"status"`
	ProviderID int                               `json:"provider_id"`
	From       *time.Time                        `json:"from"`
	To         *time.Time                        `json:"to"`
	Limit      int                               `json:"limit"`
	After      readmodel.ConsumerHistoryPosition `json:"after"`
}

func parseConsumerHistoryQuery(raw string, id int, codec *signedcursor.Codec) (admin.ConsumerHistoryQuery, error) {
	q := admin.ConsumerHistoryQuery{Limit: admin.DefaultConsumerHistoryLimit}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return q, admin.ErrInvalidConsumerHistoryQuery
	}
	for key, v := range values {
		switch key {
		case "type", "status", "provider_id", "from", "to", "limit", "cursor":
		default:
			return q, admin.ErrInvalidConsumerHistoryQuery
		}
		if len(v) != 1 || v[0] == "" {
			return q, admin.ErrInvalidConsumerHistoryQuery
		}
	}
	if token := values.Get("cursor"); token != "" {
		var p consumerHistoryCursor
		if err := codec.Decode(token, &p); err != nil || p.Version != 1 || p.ConsumerID != id {
			return q, admin.ErrInvalidConsumerHistoryQuery
		}
		q = admin.ConsumerHistoryQuery{Type: p.Type, Status: p.Status, ProviderID: p.ProviderID, From: p.From, To: p.To, Limit: p.Limit, After: &p.After}
		if err := q.Validate(); err != nil {
			return q, err
		}
	}
	for key, v := range values {
		if key == "cursor" {
			continue
		}
		previous := q
		switch key {
		case "type":
			q.Type = v[0]
		case "status":
			q.Status = v[0]
		case "provider_id":
			q.ProviderID, err = parseConsumerHistoryID(v[0])
		case "limit":
			q.Limit, err = parseConsumerHistoryID(v[0])
		case "from", "to":
			var t time.Time
			t, err = time.Parse(time.RFC3339Nano, v[0])
			if err == nil {
				t = t.UTC()
				if key == "from" {
					q.From = &t
				} else {
					q.To = &t
				}
			}
		}
		if err != nil {
			return q, admin.ErrInvalidConsumerHistoryQuery
		}
		if values.Get("cursor") != "" && !sameConsumerHistoryFilters(previous, q) {
			return q, admin.ErrInvalidConsumerHistoryQuery
		}
	}
	return q, q.Validate()
}
func sameConsumerHistoryFilters(a, b admin.ConsumerHistoryQuery) bool {
	return a.Type == b.Type && a.Status == b.Status && a.ProviderID == b.ProviderID && a.Limit == b.Limit && sameHistoryTime(a.From, b.From) && sameHistoryTime(a.To, b.To)
}
func sameHistoryTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
func parseConsumerHistoryID(raw string) (int, error) {
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, admin.ErrInvalidConsumerHistoryQuery
		}
	}
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || id < 1 {
		return 0, admin.ErrInvalidConsumerHistoryQuery
	}
	return int(id), nil
}
