package operation_funnel_handler

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/stretchr/testify/require"
)

func TestParseFunnelQueryDefaultsAndNormalizesPairedZonedPeriod(t *testing.T) {
	query, err := parseFunnelQuery("")
	require.NoError(t, err)
	require.Nil(t, query.Period)
	require.Nil(t, query.CategoryID)

	query, err = parseFunnelQuery("from=2026-09-01T00:00:00.000000001-03:00&to=2026-09-02T00:00:00%2B02:00&category_id=2147483647")
	require.NoError(t, err)
	require.Equal(t, &operation.TimeWindow{
		From: time.Date(2026, 9, 1, 3, 0, 0, 1, time.UTC),
		To:   time.Date(2026, 9, 1, 22, 0, 0, 0, time.UTC),
	}, query.Period)
	require.NotNil(t, query.CategoryID)
	require.Equal(t, 2147483647, *query.CategoryID)
}

func TestParseFunnelQueryRejectsUnknownAmbiguousAndInvalidFilters(t *testing.T) {
	for _, raw := range []string{
		"from=2026-09-01T00:00:00Z", "to=2026-09-02T00:00:00Z", "from=&to=2026-09-02T00:00:00Z",
		"from=bad&to=2026-09-02T00:00:00Z", "from=2026-09-01T00:00:00&to=2026-09-02T00:00:00",
		"from=2026-09-01T00:00:00Z&to=2026-09-02T00:00:00Z&from=2026-09-01T00:00:00Z",
		"to=2026-09-02T00:00:00Z&to=2026-09-03T00:00:00Z", "category_id=", "category_id=0", "category_id=-1",
		"category_id=+1", "category_id=2147483648", "category_id=abc", "diagnostico=true", "%zz=x",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := parseFunnelQuery(raw)
			require.ErrorIs(t, err, operation.ErrInvalidFunnelQuery)
		})
	}
}
