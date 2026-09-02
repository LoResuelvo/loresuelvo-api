package provider

import "math"

const RatingDistributionSize = 5

type RatingDistribution [RatingDistributionSize]int

type RatingStats struct {
	Total        int64
	Count        int
	Distribution RatingDistribution
}

type RatingSummary struct {
	Average float64
	Count   int
}

func (stats RatingStats) Summary() RatingSummary {
	if stats.Count <= 0 {
		return RatingSummary{}
	}

	average := float64(stats.Total) / float64(stats.Count)
	return RatingSummary{
		Average: math.Round(average*10) / 10,
		Count:   stats.Count,
	}
}
