package mercadopago

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/stretchr/testify/require"
)

func TestFakeCheckoutClientAddPaymentUsesSuppliedProcessorID(t *testing.T) {
	client := NewFakeCheckoutClient()
	want := payment.ExternalPayment{
		ID: "mp-payment-9005", SellerAccountID: "mp-juan", ExternalReference: "intent-uuid",
		Status: payment.ExternalPaymentStatusApproved, Currency: "ARS", AmountCents: 901234,
	}

	gotID := client.AddPayment(want)
	got, err := client.GetPayment(context.Background(), "access-token", want.ID)

	require.NoError(t, err)
	require.Equal(t, want.ID, gotID)
	require.Equal(t, want, got)
}

func TestFakeCheckoutClientGeneratedPaymentIDsRemainCompatible(t *testing.T) {
	client := NewFakeCheckoutClient()
	gotID := client.AddApprovedPayment("intent-uuid", "mp-juan", 901234)
	got, err := client.GetPayment(context.Background(), "access-token", gotID)

	require.NoError(t, err)
	require.Equal(t, "fake-payment-intent-uuid", gotID)
	require.Equal(t, "intent-uuid", got.ExternalReference)
	// Preserve the fake's prior default provider evidence for existing tests.
	require.Equal(t, "ARS", got.Currency)
	require.EqualValues(t, 901234, got.AmountCents)
}
