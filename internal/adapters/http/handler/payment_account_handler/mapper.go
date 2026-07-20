package payment_account_handler

import paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"

func connectionResponseFromDomain(account *paymentaccount.PaymentAccount) connectionResponse {
	return connectionResponse{
		Status:                  account.Status(),
		AccountID:               account.ExternalAccountID(),
		CanReceivePayments:      account.CanReceivePayments(),
		CanSendServiceProposals: account.CanSendServiceProposals(),
	}
}
