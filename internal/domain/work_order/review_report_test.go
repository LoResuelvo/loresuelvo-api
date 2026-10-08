package workorder_test

import (
	"strings"
	"testing"
	"time"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/require"
)

func TestReviewReportValidatesExplanation(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		valid      bool
	}{
		{"empty", "", true}, {"trimmed", "  hello  ", true}, {"500 runes", strings.Repeat("ñ", 500), true},
		{"501 runes", strings.Repeat("ñ", 501), false}, {"invalid UTF8", string([]byte{255}), false}, {"NUL", "a\x00b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := workorder.NewReviewReport(1, 2, "personal_data", tc.text, time.Now())
			if !tc.valid {
				require.ErrorIs(t, err, workorder.ErrInvalidReviewReport)
				return
			}
			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(tc.text), report.Explanation())
			require.Equal(t, "pending", report.Status())
		})
	}
}

func TestReviewReportRequiresCategoryIdentityAndTime(t *testing.T) {
	for _, tc := range []struct {
		order, actor int
		category     string
		at           time.Time
	}{
		{0, 2, "personal_data", time.Now()}, {1, 0, "personal_data", time.Now()}, {1, 2, "other", time.Now()}, {1, 2, "personal_data", time.Time{}},
	} {
		_, err := workorder.NewReviewReport(tc.order, tc.actor, tc.category, "", tc.at)
		require.ErrorIs(t, err, workorder.ErrInvalidReviewReport)
	}
}
