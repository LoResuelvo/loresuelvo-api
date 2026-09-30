package payment

import (
	"math"
	"sort"

	rm "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
)

// projectAdminPaymentProposal keeps observed gross approvals separate from contractual obligations.
// Historical anomalies are evidence, not commands to reconcile or change the contract.
func projectAdminPaymentProposal(p rm.AdminPaymentProposal) (rm.AdminPaymentBreakdown, rm.AdminPaymentSummary, map[string][]string, error) {
	b := p.Breakdown
	s := rm.AdminPaymentSummary{ApprovedAmounts: make([]rm.AdminPaymentAmount, 0), Anomalies: make([]string, 0)}
	flags := make(map[string][]string, len(p.Intents))
	validTerms := b.Currency == "ARS" && b.ServiceTotalCents > 0 && b.DepositCents > 0 && b.DepositCents <= b.ServiceTotalCents && b.PlatformFeeTotalCents >= 0 && b.PlatformFeeDueNowCents >= 0 && b.PlatformFeeDueNowCents <= b.PlatformFeeTotalCents && !b.BookingPaymentDeadline.IsZero()
	if validTerms {
		due, err := adminAddAmount(b.DepositCents, b.PlatformFeeDueNowCents)
		if err != nil {
			return b, s, nil, err
		}
		remainingService := b.ServiceTotalCents - b.DepositCents
		remainingFee := b.PlatformFeeTotalCents - b.PlatformFeeDueNowCents
		remaining, err := adminAddAmount(remainingService, remainingFee)
		if err != nil {
			return b, s, nil, err
		}
		if _, err = adminAddAmount(b.ServiceTotalCents, b.PlatformFeeTotalCents); err != nil {
			return b, s, nil, err
		}
		b.AmountDueNowCents = &due
		b.RemainingServiceBalanceCents = &remainingService
		b.RemainingPlatformFeeCents = &remainingFee
		b.RemainingAmountDueCents = &remaining
	} else {
		s.Anomalies = appendAdminFlag(s.Anomalies, "invalid_contract_terms")
	}
	totals := map[string]int64{}
	seen := map[string]bool{}
	validDeposit := false
	for _, i := range p.Intents {
		f := make([]string, 0)
		coherentIntent := false
		if validTerms {
			seller, fee, total := b.DepositCents, b.PlatformFeeDueNowCents, *b.AmountDueNowCents
			if i.Purpose == "service_balance" {
				seller, fee, total = *b.RemainingServiceBalanceCents, *b.RemainingPlatformFeeCents, *b.RemainingAmountDueCents
			}
			if i.Currency != b.Currency {
				f = appendAdminFlag(f, "intent_currency_mismatch")
			}
			if i.SellerAmountCents != seller || i.PlatformFeeCents != fee || i.TotalAmountCents != total {
				f = appendAdminFlag(f, "intent_contract_amount_mismatch")
			}
			coherentIntent = i.Currency == b.Currency && i.SellerAmountCents == seller && i.PlatformFeeCents == fee && i.TotalAmountCents == total
		}
		approvedIDs := map[string]bool{}
		approvedTotals := map[string]int64{}
		hasValidApproved := false
		for _, t := range i.Transactions {
			if t.Currency != i.Currency {
				f = appendAdminFlag(f, "transaction_currency_mismatch")
			}
			if t.AmountCents != i.TotalAmountCents {
				f = appendAdminFlag(f, "transaction_amount_mismatch")
			}
			if t.Status != "approved" {
				continue
			}
			key := t.Processor + "\x00" + t.ExternalPaymentID
			if !approvedIDs[key] {
				amount, err := adminAddAmount(approvedTotals[t.Currency], t.AmountCents)
				if err != nil {
					return b, s, nil, err
				}
				approvedTotals[t.Currency] = amount
			}
			approvedIDs[key] = true
			if t.Currency == i.Currency && t.AmountCents == i.TotalAmountCents {
				hasValidApproved = true
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			total, err := adminAddAmount(totals[t.Currency], t.AmountCents)
			if err != nil {
				return b, s, nil, err
			}
			totals[t.Currency] = total
		}
		if approvedTotals[i.Currency] > i.TotalAmountCents {
			f = appendAdminFlag(f, "approved_amount_exceeds_intent_total")
		}
		if len(approvedIDs) > 1 {
			f = appendAdminFlag(f, "multiple_approved_transactions")
		}
		if i.IntentStatus == "paid" && !hasValidApproved {
			f = appendAdminFlag(f, "paid_without_matching_approved_transaction")
		}
		if i.IntentStatus != "paid" && len(approvedIDs) > 0 {
			f = appendAdminFlag(f, "approved_transaction_intent_status_mismatch")
		}
		if coherentIntent && hasValidApproved && i.Purpose == "booking_deposit" && i.IntentStatus == "paid" {
			validDeposit = true
		}
		flags[i.ID] = f
		for _, a := range f {
			s.Anomalies = appendAdminFlag(s.Anomalies, a)
		}
	}
	currencies := make([]string, 0, len(totals))
	for c := range totals {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)
	for _, c := range currencies {
		s.ApprovedAmounts = append(s.ApprovedAmounts, rm.AdminPaymentAmount{Currency: c, AmountCents: totals[c]})
	}
	if validTerms && p.WorkOrderID != nil && p.Status == "accepted" {
		switch p.WorkOrderStatus {
		case "paid":
			s.PendingAmount = &rm.AdminPaymentAmount{Currency: b.Currency, AmountCents: 0}
		case "scheduled", "awaiting_payment":
			if validDeposit {
				s.PendingAmount = &rm.AdminPaymentAmount{Currency: b.Currency, AmountCents: *b.RemainingAmountDueCents}
			}
		}
	}
	if s.PendingAmount == nil {
		s.Anomalies = appendAdminFlag(s.Anomalies, "contractual_pending_unavailable")
	}
	return b, s, flags, nil
}
func adminAddAmount(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > math.MaxInt64-b {
		return 0, ErrAdminPaymentAmountOverflow
	}
	return a + b, nil
}
func appendAdminFlag(flags []string, flag string) []string {
	for _, s := range flags {
		if s == flag {
			return flags
		}
	}
	return append(flags, flag)
}
