package service_proposal_handler

import "time"

type createServiceProposalRequest struct {
	ConsumerID               int        `json:"consumer_id" binding:"required"`
	Amount                   string     `json:"amount" binding:"required"`
	ScheduledOn              *time.Time `json:"scheduled_on" binding:"required"`
	Description              string     `json:"description" binding:"required"`
	EstimatedDurationMinutes *int       `json:"estimated_duration_minutes" binding:"required"`
}
