package payment

import (
	"fmt"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

const (
	bookingCheckoutLockNamespace        = 2118
	serviceBalanceCheckoutLockNamespace = 2119
	externalPaymentLockNamespace        = 2120
)

func BookingCheckoutLockKey(serviceProposalID int) LockKey {
	return LockKey{
		Namespace: bookingCheckoutLockNamespace,
		Resource:  fmt.Sprintf("%d", serviceProposalID),
	}
}

func ServiceBalanceCheckoutLockKey(workOrderID int) LockKey {
	return LockKey{
		Namespace: serviceBalanceCheckoutLockNamespace,
		Resource:  fmt.Sprintf("%d", workOrderID),
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
