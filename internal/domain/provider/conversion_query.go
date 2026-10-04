package provider

import (
	"errors"
	"time"
)

var ErrInvalidConversionQuery = errors.New("invalid conversion query")

type ConversionQueryInput struct{ From, To *time.Time }
type ConversionQuery struct{ From, To time.Time }

func (input ConversionQueryInput) resolve(now time.Time) (ConversionQuery, error) {
	from, to, valid := resolveStatisticsPeriod(now, input.From, input.To)
	if !valid {
		return ConversionQuery{}, ErrInvalidConversionQuery
	}
	return ConversionQuery{from, to}, nil
}
