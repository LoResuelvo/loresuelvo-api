package evals

import (
	"errors"
	"fmt"
	"html"
	"math"
	"sort"
	"strconv"
	"strings"
)

// ErrInvalidCampaignChartData indicates that an aggregate contains impossible
// or non-finite values. Charts fail closed rather than filling missing values.
var ErrInvalidCampaignChartData = errors.New("invalid campaign chart data")

// CoverageChartSeries contains counts for one model. Counts refer to planned
// campaign slots, not independent observations or retry attempts.
type CoverageChartSeries struct {
	Name            string
	ExpectedSlots   int
	TerminalSlots   int
	ExecutedSlots   int
	FailedSlots     int
	SemanticUnknown int
	MissingSlots    int
}

// QualityChartSeries contains one aggregate quality metric. Value is nil when
// the metric is not observed; Observed and Expected preserve its denominator.
type QualityChartSeries struct {
	Name    string
	Metric  string
	Summary MetricSummary
}

// OperationsChartSeries contains measured operational data for one model.
// Nil values remain visibly unknown in the SVG.
type OperationsChartSeries struct {
	Name       string
	Operations OperationalSummary
}

// RankingChartSeries contains one ranking metric for a model or offline
// baseline. Baselines are displayed as baselines, never as model trials.
type RankingChartSeries struct {
	Name     string
	Metric   string
	Summary  MetricSummary
	Baseline bool
}

// RenderCoverageSVG renders deterministic slot coverage. It reports missing,
// failed and semantic-unknown counts explicitly and never infers them as zero.
func RenderCoverageSVG(series []CoverageChartSeries) ([]byte, error) {
	if err := validateCoverage(series); err != nil {
		return nil, err
	}
	series = cloneCoverageSorted(series)
	const rowHeight, top = 58, 54
	height := top + max(1, len(series))*rowHeight + 24
	var b strings.Builder
	beginSVG(&b, 900, height, "Campaign coverage")
	textSVG(&b, 24, 30, "Campaign coverage (planned slots)", 18, "title")
	textSVG(&b, 24, 47, "Counts are slots; semantic unknown is not an execution failure.", 11, "subtitle")
	for i, item := range series {
		y := top + i*rowHeight
		textSVG(&b, 24, y+17, item.Name, 13, "label")
		textSVG(&b, 24, y+34, fmt.Sprintf("terminal %d/%d · executed %d · failed %d · missing %d · semantic unknown %d", item.TerminalSlots, item.ExpectedSlots, item.ExecutedSlots, item.FailedSlots, item.MissingSlots, item.SemanticUnknown), 11, "value")
		bar := 190
		if item.ExpectedSlots > 0 {
			bar = max(1, min(bar, item.ExpectedSlots*12))
		}
		x := 220
		drawCountBar(&b, x, y+7, bar, item.ExecutedSlots, item.ExpectedSlots, "#2f855a", "executed")
		drawCountBar(&b, x+bar+8, y+7, bar, item.FailedSlots, item.ExpectedSlots, "#c53030", "failed")
		drawCountBar(&b, x+2*(bar+8), y+7, bar, item.MissingSlots, item.ExpectedSlots, "#718096", "missing")
	}
	legendSVG(&b, 24, height-14, []legendItem{{"executed", "#2f855a"}, {"failed", "#c53030"}, {"missing", "#718096"}})
	endSVG(&b)
	return []byte(b.String()), nil
}

// RenderQualitySVG renders metric means with their observed/expected
// denominator. Nil means are rendered as "unknown", not as zero.
func RenderQualitySVG(series []QualityChartSeries) ([]byte, error) {
	if err := validateQuality(series); err != nil {
		return nil, err
	}
	series = cloneQualitySorted(series)
	const rowHeight, top = 50, 54
	height := top + max(1, len(series))*rowHeight + 24
	var b strings.Builder
	beginSVG(&b, 900, height, "Campaign quality")
	textSVG(&b, 24, 30, "Campaign quality", 18, "title")
	textSVG(&b, 24, 47, "Mean is shown only when observed; n is evaluated/expected observations.", 11, "subtitle")
	for i, item := range series {
		y := top + i*rowHeight
		label := item.Name
		if item.Metric != "" {
			label += " · " + item.Metric
		}
		textSVG(&b, 24, y+15, label, 12, "label")
		if item.Summary.Mean == nil {
			textSVG(&b, 420, y+15, "unknown", 12, "unknown")
		} else {
			value := *item.Summary.Mean
			width := int(math.Round(min(1, max(0, value)) * 360))
			b.WriteString(fmt.Sprintf(`<rect x="420" y="%d" width="%d" height="16" fill="#2b6cb0"/>`, y+2, width))
			textSVG(&b, 790, y+15, formatFloat(value), 12, "value")
		}
		textSVG(&b, 24, y+34, fmt.Sprintf("n=%d/%d · base cases=%d/%d", item.Summary.Observations, item.Summary.ExpectedObservations, item.Summary.EvaluatedBaseCases, item.Summary.ExpectedObservations), 11, "value")
	}
	endSVG(&b)
	return []byte(b.String()), nil
}

// RenderOperationsSVG renders latency, token totals and cost as an explicit
// table. Unknown measurements are not substituted with zero.
func RenderOperationsSVG(series []OperationsChartSeries) ([]byte, error) {
	if err := validateOperations(series); err != nil {
		return nil, err
	}
	series = cloneOperationsSorted(series)
	const rowHeight, top = 76, 54
	height := top + max(1, len(series))*rowHeight + 24
	var b strings.Builder
	beginSVG(&b, 1000, height, "Campaign operations")
	textSVG(&b, 24, 30, "Campaign operations", 18, "title")
	textSVG(&b, 24, 47, "Only provider-reported usage and measured latency are plotted.", 11, "subtitle")
	for i, item := range series {
		y := top + i*rowHeight
		o := item.Operations
		textSVG(&b, 24, y+15, item.Name, 13, "label")
		textSVG(&b, 24, y+34, fmt.Sprintf("requests=%d · latency p50=%s p95=%s (n=%d/%d)", o.Requests, optionalFloat(o.LatencyP50Millis), optionalFloat(o.LatencyP95Millis), o.LatencyObservations, o.Requests), 11, "value")
		textSVG(&b, 24, y+52, fmt.Sprintf("tokens input=%s output=%s thought=%s total=%s · cost=%s", optionalInt(o.InputTokens.ObservedTotal), optionalInt(o.OutputTokens.ObservedTotal), optionalInt(o.ThoughtTokens.ObservedTotal), optionalInt(o.TotalTokens.ObservedTotal), optionalFloat(o.Cost)), 11, "value")
	}
	endSVG(&b)
	return []byte(b.String()), nil
}

// RenderRankingSVG renders model and baseline ranking metrics side by side in
// one deterministic chart. Baseline observations are labelled as such.
func RenderRankingSVG(series []RankingChartSeries) ([]byte, error) {
	if err := validateRanking(series); err != nil {
		return nil, err
	}
	series = cloneRankingSorted(series)
	const rowHeight, top = 50, 54
	height := top + max(1, len(series))*rowHeight + 24
	var b strings.Builder
	beginSVG(&b, 900, height, "Ranking quality")
	textSVG(&b, 24, 30, "Ranking quality", 18, "title")
	textSVG(&b, 24, 47, "Model and offline baseline metrics; repeated trials are not independent samples.", 11, "subtitle")
	for i, item := range series {
		y := top + i*rowHeight
		label := item.Name
		if item.Baseline {
			label += " [baseline]"
		}
		if item.Metric != "" {
			label += " · " + item.Metric
		}
		textSVG(&b, 24, y+15, label, 12, "label")
		if item.Summary.Mean == nil {
			textSVG(&b, 420, y+15, "unknown", 12, "unknown")
		} else {
			value := *item.Summary.Mean
			width := int(math.Round(min(1, max(0, value)) * 360))
			color := "#805ad5"
			if item.Baseline {
				color = "#718096"
			}
			b.WriteString(fmt.Sprintf(`<rect x="420" y="%d" width="%d" height="16" fill="%s"/>`, y+2, width, color))
			textSVG(&b, 790, y+15, formatFloat(value), 12, "value")
		}
		textSVG(&b, 24, y+34, fmt.Sprintf("n=%d/%d · base cases=%d/%d", item.Summary.Observations, item.Summary.ExpectedObservations, item.Summary.EvaluatedBaseCases, item.Summary.ExpectedObservations), 11, "value")
	}
	endSVG(&b)
	return []byte(b.String()), nil
}

func validateCoverage(items []CoverageChartSeries) error {
	for _, item := range items {
		if item.Name == "" || item.ExpectedSlots < 0 || item.TerminalSlots < 0 || item.ExecutedSlots < 0 || item.FailedSlots < 0 || item.SemanticUnknown < 0 || item.MissingSlots < 0 || item.TerminalSlots > item.ExpectedSlots || item.ExecutedSlots > item.TerminalSlots || item.FailedSlots > item.TerminalSlots || item.ExecutedSlots+item.FailedSlots > item.TerminalSlots || item.MissingSlots > item.ExpectedSlots-item.TerminalSlots {
			return fmt.Errorf("%w: coverage series %q", ErrInvalidCampaignChartData, item.Name)
		}
	}
	return nil
}
func validateQuality(items []QualityChartSeries) error {
	for _, item := range items {
		if item.Name == "" || item.Summary.Observations < 0 || item.Summary.ExpectedObservations < 0 || item.Summary.Observations > item.Summary.ExpectedObservations || item.Summary.EvaluatedBaseCases < 0 || !finitePtr(item.Summary.Mean) {
			return fmt.Errorf("%w: quality series %q", ErrInvalidCampaignChartData, item.Name)
		}
	}
	return nil
}
func validateOperations(items []OperationsChartSeries) error {
	for _, item := range items {
		o := item.Operations
		if item.Name == "" || o.Requests < 0 || o.LatencyObservations < 0 || o.LatencyUnknownRequests < 0 || o.LatencyObservations+o.LatencyUnknownRequests > o.Requests || !validNonNegativeFloat(o.LatencyP50Millis) || !validNonNegativeFloat(o.LatencyP95Millis) || !validNonNegativeFloat(o.Cost) || !validTokenSummary(o.InputTokens, o.Requests) || !validTokenSummary(o.OutputTokens, o.Requests) || !validTokenSummary(o.ThoughtTokens, o.Requests) || !validTokenSummary(o.TotalTokens, o.Requests) {
			return fmt.Errorf("%w: operations series %q", ErrInvalidCampaignChartData, item.Name)
		}
	}
	return nil
}
func validateRanking(items []RankingChartSeries) error {
	return validateQuality(func() []QualityChartSeries {
		out := make([]QualityChartSeries, len(items))
		for i, item := range items {
			out[i] = QualityChartSeries{Name: item.Name, Metric: item.Metric, Summary: item.Summary}
		}
		return out
	}())
}
func finitePtr(value *float64) bool {
	return value == nil || (!math.IsNaN(*value) && !math.IsInf(*value, 0))
}
func validNonNegativeFloat(value *float64) bool {
	return value == nil || (finitePtr(value) && *value >= 0)
}
func validTokenSummary(summary TokenSummary, requests int) bool {
	if summary.KnownRequests < 0 || summary.UnknownRequests < 0 || summary.KnownRequests+summary.UnknownRequests > requests {
		return false
	}
	return summary.ObservedTotal == nil || *summary.ObservedTotal >= 0
}
func cloneCoverageSorted(v []CoverageChartSeries) []CoverageChartSeries {
	out := append([]CoverageChartSeries(nil), v...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func cloneQualitySorted(v []QualityChartSeries) []QualityChartSeries {
	out := append([]QualityChartSeries(nil), v...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Name+"\x00"+out[i].Metric, out[j].Name+"\x00"+out[j].Metric
		return a < b
	})
	return out
}
func cloneOperationsSorted(v []OperationsChartSeries) []OperationsChartSeries {
	out := append([]OperationsChartSeries(nil), v...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func cloneRankingSorted(v []RankingChartSeries) []RankingChartSeries {
	out := append([]RankingChartSeries(nil), v...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Name+"\x00"+out[i].Metric+"\x00"+strconv.FormatBool(out[i].Baseline), out[j].Name+"\x00"+out[j].Metric+"\x00"+strconv.FormatBool(out[j].Baseline)
		return a < b
	})
	return out
}

func beginSVG(b *strings.Builder, width, height int, label string) {
	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s"><style>.title{font:600 18px sans-serif;fill:#1a202c}.subtitle,.value{font:11px sans-serif;fill:#4a5568}.label{font:600 12px sans-serif;fill:#2d3748}.unknown{font:12px sans-serif;fill:#718096}</style>`, width, height, width, height, html.EscapeString(label)))
}
func endSVG(b *strings.Builder) { b.WriteString(`</svg>` + "\n") }
func textSVG(b *strings.Builder, x, y int, text string, size int, class string) {
	b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" class="%s" font-size="%d">%s</text>`, x, y, class, size, html.EscapeString(text)))
}
func drawCountBar(b *strings.Builder, x, y, width, count, expected int, color, label string) {
	fraction := 0.0
	if expected > 0 {
		fraction = float64(count) / float64(expected)
		if fraction > 1 {
			fraction = 1
		}
	}
	fill := int(math.Round(float64(width) * fraction))
	b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="14" fill="#edf2f7"/><rect x="%d" y="%d" width="%d" height="14" fill="%s"><title>%s: %d</title></rect>`, x, y, width, x, y, fill, color, html.EscapeString(label), count))
}

type legendItem struct{ label, color string }

func legendSVG(b *strings.Builder, x, y int, items []legendItem) {
	for _, item := range items {
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="10" height="10" fill="%s"/><text x="%d" y="%d" class="value">%s</text>`, x, y-9, item.color, x+15, y, html.EscapeString(item.label)))
		x += 95
	}
}
func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
func optionalFloat(v *float64) string {
	if v == nil {
		return "unknown"
	}
	return formatFloat(*v)
}
func optionalInt(v *int64) string {
	if v == nil {
		return "unknown"
	}
	return strconv.FormatInt(*v, 10)
}

// TrialVariabilitySeries captures within-case variation across repeated trials.
// Repeated trials are paired measurements, not independent samples.
type TrialVariabilitySeries struct {
	Name                string
	Metric              string
	ExpectedBaseCases   int
	EvaluatedBaseCases  int
	ComparableBaseCases int
	VariableBaseCases   int
	MeanWithinCaseRange *float64
	MaxWithinCaseRange  *float64
}

// RenderVariabilitySVG renders within-case trial variability with explicit
// comparable/variable denominators. It never presents a population CI.
func RenderVariabilitySVG(series []TrialVariabilitySeries) ([]byte, error) {
	if err := validateVariability(series); err != nil {
		return nil, err
	}
	series = append([]TrialVariabilitySeries(nil), series...)
	sort.SliceStable(series, func(i, j int) bool {
		a, b := series[i].Name+"\x00"+series[i].Metric, series[j].Name+"\x00"+series[j].Metric
		return a < b
	})
	const rowHeight, top = 58, 54
	height := top + max(1, len(series))*rowHeight + 24
	var b strings.Builder
	beginSVG(&b, 1000, height, "Trial variability")
	textSVG(&b, 24, 30, "Trial variability", 18, "title")
	textSVG(&b, 24, 47, "Within-case repeated-trial ranges; no population uncertainty is inferred.", 11, "subtitle")
	for i, item := range series {
		y := top + i*rowHeight
		label := item.Name
		if item.Metric != "" {
			label += " · " + item.Metric
		}
		textSVG(&b, 24, y+16, label, 12, "label")
		textSVG(&b, 24, y+35, fmt.Sprintf("base cases evaluated=%d/%d · comparable=%d · variable=%d", item.EvaluatedBaseCases, item.ExpectedBaseCases, item.ComparableBaseCases, item.VariableBaseCases), 11, "value")
		textSVG(&b, 24, y+52, fmt.Sprintf("mean within-case range=%s · max within-case range=%s", optionalFloat(item.MeanWithinCaseRange), optionalFloat(item.MaxWithinCaseRange)), 11, "value")
	}
	endSVG(&b)
	return []byte(b.String()), nil
}

func validateVariability(items []TrialVariabilitySeries) error {
	for _, item := range items {
		if item.Name == "" || item.ExpectedBaseCases < 0 || item.EvaluatedBaseCases < 0 || item.ComparableBaseCases < 0 || item.VariableBaseCases < 0 || item.EvaluatedBaseCases > item.ExpectedBaseCases || item.ComparableBaseCases > item.EvaluatedBaseCases || item.VariableBaseCases > item.ComparableBaseCases || !validNonNegativeFloat(item.MeanWithinCaseRange) || !validNonNegativeFloat(item.MaxWithinCaseRange) {
			return fmt.Errorf("%w: variability series %q", ErrInvalidCampaignChartData, item.Name)
		}
		if item.MeanWithinCaseRange != nil && item.MaxWithinCaseRange != nil && *item.MeanWithinCaseRange > *item.MaxWithinCaseRange {
			return fmt.Errorf("%w: variability range ordering for %q", ErrInvalidCampaignChartData, item.Name)
		}
	}
	return nil
}
