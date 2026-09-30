package operation_funnel_handler

import (
	"net/url"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
)

func parseFunnelQuery(raw string) (operation.FunnelQuery, error) {
	query := operation.FunnelQuery{}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return query, operation.ErrInvalidFunnelQuery
	}
	for key, entries := range values {
		switch key {
		case "from", "to", "category_id":
		default:
			return query, operation.ErrInvalidFunnelQuery
		}
		if len(entries) != 1 || entries[0] == "" {
			return query, operation.ErrInvalidFunnelQuery
		}
	}

	fromValue, hasFrom := values["from"]
	toValue, hasTo := values["to"]
	if hasFrom != hasTo {
		return query, operation.ErrInvalidFunnelQuery
	}
	if hasFrom {
		from, parseErr := time.Parse(time.RFC3339Nano, fromValue[0])
		if parseErr != nil {
			return query, operation.ErrInvalidFunnelQuery
		}
		to, parseErr := time.Parse(time.RFC3339Nano, toValue[0])
		if parseErr != nil {
			return query, operation.ErrInvalidFunnelQuery
		}
		query.Period = &operation.TimeWindow{From: from.UTC(), To: to.UTC()}
	}

	if categoryValues, exists := values["category_id"]; exists {
		categoryID, parseErr := parsePositiveCategoryID(categoryValues[0])
		if parseErr != nil {
			return query, operation.ErrInvalidFunnelQuery
		}
		query.CategoryID = &categoryID
	}
	return query, nil
}

func parsePositiveCategoryID(value string) (int, error) {
	if value == "" {
		return 0, operation.ErrInvalidFunnelQuery
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, operation.ErrInvalidFunnelQuery
		}
	}
	categoryID, err := strconv.ParseInt(value, 10, 32)
	if err != nil || categoryID <= 0 {
		return 0, operation.ErrInvalidFunnelQuery
	}
	return int(categoryID), nil
}
