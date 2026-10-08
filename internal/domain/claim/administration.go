package claim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

// ResolutionInput contains only choices made by the operator; monetary constants belong to the business.
type ResolutionInput struct {
	Type                 ResolutionType
	Reasoning            string
	SuggestedAmountMinor *int64
}

func (input ResolutionInput) Normalize() (ResolutionInput, error) {
	if !utf8.ValidString(input.Reasoning) || strings.ContainsRune(input.Reasoning, 0) {
		return ResolutionInput{}, ErrInvalidResolution
	}
	input.Reasoning = strings.TrimSpace(input.Reasoning)
	if n := utf8.RuneCountInString(input.Reasoning); n < 1 || n > 5000 {
		return ResolutionInput{}, ErrInvalidResolution
	}
	switch input.Type {
	case ResolutionTypeConsumerFavor, ResolutionTypeProviderFavor, ResolutionTypeAgreement, ResolutionTypeWithoutMerit:
	default:
		return ResolutionInput{}, ErrInvalidResolution
	}
	if input.SuggestedAmountMinor != nil {
		amount := *input.SuggestedAmountMinor
		if amount <= 0 {
			return ResolutionInput{}, ErrInvalidResolution
		}
		input.SuggestedAmountMinor = &amount
	}
	return input, nil
}

func administrationFingerprint(id int, action string, input ResolutionInput) string {
	data, _ := json.Marshal(struct {
		ID     int
		Action string
		Input  ResolutionInput
	}{id, action, input})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (c *Claim) StartReview(operatorID int, now time.Time) (*Action, error) {
	if operatorID <= 0 {
		return nil, ErrForbidden
	}
	now = now.UTC().Truncate(time.Microsecond)
	if c.Status != StatusOpen || now.Before(c.CreatedOn) {
		return nil, ErrInvalidTransition
	}
	c.Status = StatusInReview
	c.ReviewStartedOn = &now
	c.Actions = append(c.Actions, Action{Type: "review_started", ActorID: operatorID, ActorParty: PartyOperator, CreatedOn: now})
	return &c.Actions[len(c.Actions)-1], nil
}

func (c *Claim) Resolve(operatorID int, input ResolutionInput, now time.Time) (*Action, error) {
	input, err := input.Normalize()
	if err != nil {
		return nil, err
	}
	if operatorID <= 0 {
		return nil, ErrForbidden
	}
	now = now.UTC().Truncate(time.Microsecond)
	if c.Status != StatusInReview || c.ReviewStartedOn == nil || now.Before(*c.ReviewStartedOn) {
		return nil, ErrInvalidTransition
	}
	c.Status = StatusResolved
	if input.Type == ResolutionTypeWithoutMerit {
		c.Status = StatusDismissed
	}
	c.ClosedOn = &now
	c.Resolution = &Resolution{Type: input.Type, Reasoning: input.Reasoning, ResolvedOn: now}
	if input.SuggestedAmountMinor != nil {
		c.Resolution.SuggestedCompensation = &SuggestedCompensation{AmountMinor: *input.SuggestedAmountMinor, Currency: "ARS", Unit: "minor"}
	}
	c.Actions = append(c.Actions, Action{Type: "resolved", ActorID: operatorID, ActorParty: PartyOperator, CreatedOn: now})
	return &c.Actions[len(c.Actions)-1], nil
}
