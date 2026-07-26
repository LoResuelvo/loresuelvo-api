package payment

import (
	"fmt"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

const (
	bookingCheckoutLockNamespace = 2118
	externalPaymentLockNamespace = 2120
)

func BookingCheckoutLockKey(serviceProposalID int) LockKey {
	return LockKey{
		Namespace: bookingCheckoutLockNamespace,
		Resource:  fmt.Sprintf("%d", serviceProposalID),
	}
}

func ExternalPaymentLockKey(
	processor paymentaccount.PaymentProvider,
	externalPaymentID string,
) LockKey {
	return LockKey{
		Namespace: externalPaymentLockNamespace,
		Resource:  string(processor) + ":" + externalPaymentID,
	}
}
