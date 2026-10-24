package ledger

import (
	"fmt"

	"github.com/google/uuid"
)

// checks a structurally valid transaction against the current state of its accounts
func CheckPosting(tx Transaction, accounts map[uuid.UUID]Account, balances map[uuid.UUID]int64) error {
	var currency Currency
	for _, entry := range tx.Entries {
		account, ok := accounts[entry.AccountID]
		if !ok {
			return fmt.Errorf("%w: %s", ErrAccountNotFound, entry.AccountID)
		}
		if currency == "" {
			currency = account.Currency
		} else if account.Currency != currency {
			return fmt.Errorf("%w: %s and %s in one transaction", ErrCurrencyMismatch, currency, account.Currency)
		}
	}

	for _, entry := range tx.Entries {
		account := accounts[entry.AccountID]
		balance := balances[entry.AccountID]

		newBalance, ok := addChecked(balance, entry.Amount)
		if !ok {
			return fmt.Errorf("%w: balance of account %s would overflow", ErrInvalidInput, entry.AccountID)
		}
		if newBalance < 0 && !account.AllowNegative {
			return fmt.Errorf("%w: account %s has %d, needs %d", ErrInsufficientFunds, entry.AccountID, balance, -entry.Amount)
		}
	}

	return nil
}
