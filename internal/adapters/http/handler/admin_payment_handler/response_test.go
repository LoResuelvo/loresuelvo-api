package admin_payment_handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/stretchr/testify/require"
)

func TestPaymentResponseKeepsEvidenceAndUnavailableSettlementNull(t *testing.T) {
	verified := time.Date(2026, 9, 20, 12, 0, 0, 0, time.FixedZone("offset", 3*60*60))
	model := readmodel.AdminPayment{
		ID: "123e4567-e89b-12d3-a456-426614174000", Purpose: "booking_deposit", IntentStatus: "paid",
		CreatedOn: verified, UpdatedOn: verified,
		Transactions: []readmodel.AdminPaymentTransaction{{
			ID: 9, Processor: "mercado_pago", ExternalPaymentID: "mp-9", Status: "approved",
			Currency: "ARS", AmountCents: 901234, VerifiedOn: verified, CreatedOn: verified, UpdatedOn: verified,
		}},
		Summary: readmodel.AdminPaymentSummary{
			ApprovedAmounts: []readmodel.AdminPaymentAmount{{Currency: "ARS", AmountCents: 901234}},
			Anomalies:       []string{},
		},
	}
	encoded, err := json.Marshal(paymentResponseFromModel(model))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(encoded, &body))
	require.Equal(t, "2026-09-20T09:00:00Z", body["created_on"])
	require.Equal(t, []any{}, body["anomalies"])
	require.Equal(t, []any{}, body["summary"].(map[string]any)["anomalies"])
	require.Nil(t, body["summary"].(map[string]any)["pending_amount"])
	require.Equal(t, float64(901234), body["summary"].(map[string]any)["approved_amounts"].([]any)[0].(map[string]any)["amount_cents"])
	transaction := body["transactions"].([]any)[0].(map[string]any)
	require.Equal(t, "mp-9", transaction["external_payment_id"])
	require.Nil(t, transaction["processor_fee_cents"])
	require.Nil(t, transaction["net_settlement_cents"])
}

func TestResponseFromPageReturnsNonNullEmptyCollectionsAndSignedCursor(t *testing.T) {
	codec := mustCodec(t, cursorPurpose)
	query := paymentQueryForTest()
	result, err := responseFromPage(&readmodel.AdminPaymentPage{Payments: nil, Limit: query.Limit}, codec, query)
	require.NoError(t, err)
	require.NotNil(t, result.Payments)
	require.Empty(t, result.Payments)
	require.Nil(t, result.Page.NextCursor)

	position := readmodel.AdminPaymentPosition{CreatedOn: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), ID: query.PaymentIntentID}
	result, err = responseFromPage(&readmodel.AdminPaymentPage{Payments: []readmodel.AdminPayment{{ID: position.ID}}, Limit: query.Limit, Next: &position}, codec, query)
	require.NoError(t, err)
	require.NotNil(t, result.Page.NextCursor)
	parsed, err := parseQuery("cursor="+*result.Page.NextCursor, codec)
	require.NoError(t, err)
	require.Equal(t, &position, parsed.After)
}

func TestPaymentResponseUsesEmptyArraysAndNullForUnavailableContractualValues(t *testing.T) {
	encoded, err := json.Marshal(paymentResponseFromModel(readmodel.AdminPayment{}))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(encoded, &body))
	require.Equal(t, []any{}, body["transactions"])
	require.Equal(t, []any{}, body["anomalies"])
	require.Equal(t, []any{}, body["summary"].(map[string]any)["approved_amounts"])
	require.Equal(t, []any{}, body["summary"].(map[string]any)["anomalies"])
	require.Nil(t, body["breakdown"].(map[string]any)["amount_due_now_cents"])
	require.Nil(t, body["breakdown"].(map[string]any)["remaining_service_balance_cents"])
	require.Nil(t, body["breakdown"].(map[string]any)["remaining_platform_fee_cents"])
	require.Nil(t, body["breakdown"].(map[string]any)["remaining_amount_due_cents"])
}

func paymentQueryForTest() payment.AdminPaymentQuery {
	return payment.AdminPaymentQuery{PaymentIntentID: "123e4567-e89b-12d3-a456-426614174000", Limit: 20}
}
