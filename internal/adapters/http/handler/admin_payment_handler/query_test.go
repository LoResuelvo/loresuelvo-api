package admin_payment_handler

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/stretchr/testify/require"
)

var testSigningKey = []byte(strings.Repeat("k", 32))

func TestParseQueryRejectsUnknownDuplicateAndEmptyParameters(t *testing.T) {
	codec, err := signedcursor.New(testSigningKey, cursorPurpose)
	require.NoError(t, err)
	for _, raw := range []string{
		"unknown=x", "payment_intent_id=", "limit=20&limit=20", "purpose=booking_deposit&purpose=booking_deposit",
		"service_proposal_id=+1", "service_proposal_id=2147483648", "limit=0", "limit=101", "limit=-1",
		"purpose=refund", "intent_status=approved", "payment_intent_id=not-a-uuid", "external_reference=not-a-uuid",
		"created_from=bad", "created_to=", "created_from=2026-09-21T00:00:00Z&created_to=2026-09-20T00:00:00Z",
		"cursor=invalid", "%zz=x",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := parseQuery(raw, codec)
			require.ErrorIs(t, err, payment.ErrInvalidAdminPaymentQuery)
		})
	}
}

func TestParseQueryDefaultsAndKeepsHistoricalStatusesFilterable(t *testing.T) {
	codec, err := signedcursor.New(testSigningKey, cursorPurpose)
	require.NoError(t, err)
	query, err := parseQuery("", codec)
	require.NoError(t, err)
	require.Equal(t, payment.DefaultAdminPaymentLimit, query.Limit)
	for _, status := range []string{"requires_checkout", "checkout_ready", "processing", "paid", "rejected", "expired", "cancelled", "refunded", "disputed", "payment_mismatch"} {
		query, err := parseQuery("intent_status="+status, codec)
		require.NoError(t, err)
		require.Equal(t, status, query.IntentStatus)
	}
}

func TestAdminPaymentCursorCarriesCompleteQueryAndRejectsChanges(t *testing.T) {
	codec, err := signedcursor.New(testSigningKey, cursorPurpose)
	require.NoError(t, err)
	from := time.Date(2026, 9, 20, 0, 0, 0, 1, time.FixedZone("offset", 3*60*60)).UTC()
	to := from.Add(time.Hour)
	query := payment.AdminPaymentQuery{
		PaymentIntentID: "123e4567-e89b-12d3-a456-426614174000", ExternalReference: "123e4567-e89b-12d3-a456-426614174000",
		ExternalPaymentID: "external-42", ServiceProposalID: 23, ConsumerEmail: "ANA@EXAMPLE", ProviderEmail: "juan@example",
		Purpose: payment.PurposeBookingDeposit, IntentStatus: "payment_mismatch", CreatedFrom: &from, CreatedTo: &to, Limit: 2,
	}
	position := readmodel.AdminPaymentPosition{CreatedOn: to, ID: "123e4567-e89b-12d3-a456-426614174001"}
	token, err := codec.Encode(cursorFromQuery(query, position))
	require.NoError(t, err)
	parsed, err := parseQuery("cursor="+url.QueryEscape(token), codec)
	require.NoError(t, err)
	require.Equal(t, query.PaymentIntentID, parsed.PaymentIntentID)
	require.Equal(t, query.ExternalReference, parsed.ExternalReference)
	require.Equal(t, query.ExternalPaymentID, parsed.ExternalPaymentID)
	require.Equal(t, query.ServiceProposalID, parsed.ServiceProposalID)
	require.Equal(t, query.ConsumerEmail, parsed.ConsumerEmail)
	require.Equal(t, query.ProviderEmail, parsed.ProviderEmail)
	require.Equal(t, query.Purpose, parsed.Purpose)
	require.Equal(t, query.IntentStatus, parsed.IntentStatus)
	require.Equal(t, query.Limit, parsed.Limit)
	require.Equal(t, &position, parsed.After)
	require.Equal(t, query.CreatedFrom, parsed.CreatedFrom)
	require.Equal(t, query.CreatedTo, parsed.CreatedTo)

	compatible := "cursor=" + url.QueryEscape(token) + "&created_from=" + url.QueryEscape(from.Format(time.RFC3339Nano)) + "&limit=2"
	_, err = parseQuery(compatible, codec)
	require.NoError(t, err)
	for _, changed := range []string{
		"&payment_intent_id=123e4567-e89b-12d3-a456-426614174002", "&external_reference=123e4567-e89b-12d3-a456-426614174002",
		"&external_payment_id=external-43", "&service_proposal_id=24", "&consumer_email=beatriz", "&provider_email=luis",
		"&purpose=service_balance", "&intent_status=paid", "&created_to=2026-09-22T00:00:00Z", "&limit=3",
	} {
		_, err := parseQuery("cursor="+url.QueryEscape(token)+changed, codec)
		require.ErrorIs(t, err, payment.ErrInvalidAdminPaymentQuery, changed)
	}
	_, err = parseQuery("cursor="+url.QueryEscape(token), mustCodec(t, "another-purpose"))
	require.ErrorIs(t, err, payment.ErrInvalidAdminPaymentQuery)
	_, err = parseQuery("cursor="+url.QueryEscape(token[:len(token)-2]+"aa"), codec)
	require.ErrorIs(t, err, payment.ErrInvalidAdminPaymentQuery)
}

func mustCodec(t *testing.T, purpose string) *signedcursor.Codec {
	t.Helper()
	codec, err := signedcursor.New(testSigningKey, purpose)
	require.NoError(t, err)
	return codec
}
