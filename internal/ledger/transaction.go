package ledger

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxAmount = 1<<53 - 1
const maxEntries = 100
const maxDescriptionLength = 500

var ErrUnbalanced = errors.New("unbalanced transaction")
var ErrCurrencyMismatch = errors.New("currency mismatch")
var ErrInsufficientFunds = errors.New("insufficient funds")
var ErrAccountNotFound = errors.New("account not found")

type Entry struct {
	ID        uuid.UUID
	AccountID uuid.UUID
	Amount    int64
}

type Transaction struct {
	ID          uuid.UUID
	Seq         int64
	Description string
	PostedAt    time.Time
	Entries     []Entry
}

func addChecked(a, b int64) (int64, bool) {
	sum := a + b
	if (b > 0 && sum < a) || (b < 0 && sum > a) {
		return sum, false
	}
	return sum, true
}

func NewTransaction(id uuid.UUID, now time.Time, desc string, entries []Entry) (Transaction, error) {
	desc = strings.TrimSpace(desc)

	if utf8.RuneCountInString(desc) > maxDescriptionLength {
		return Transaction{}, fmt.Errorf("%w: description too long", ErrInvalidInput)
	}

	if len(entries) < 2 {
		return Transaction{}, fmt.Errorf("%w: fewer than 2 entries", ErrInvalidInput)
	}
	if len(entries) > maxEntries {
		return Transaction{}, fmt.Errorf("%w: more than %d entries", ErrInvalidInput, maxEntries)
	}

	seen := make(map[uuid.UUID]struct{}, len(entries))
	var sum int64

	for i, entry := range entries {
		if entry.Amount == 0 {
			return Transaction{}, fmt.Errorf("%w: entry %d has amount zero", ErrInvalidInput, i)
		}
		if entry.Amount > maxAmount || entry.Amount < -maxAmount {
			return Transaction{}, fmt.Errorf("%w: entry %d has amount too large/small", ErrInvalidInput, i)
		}
		if _, ok := seen[entry.AccountID]; ok {
			return Transaction{}, fmt.Errorf("%w: account %s appears more than once", ErrInvalidInput, entry.AccountID)
		}
		seen[entry.AccountID] = struct{}{}

		var ok bool
		sum, ok = addChecked(sum, entry.Amount)
		if !ok {
			return Transaction{}, fmt.Errorf("%w: entry amounts overflow", ErrInvalidInput)
		}
	}

	// the key here! sum should never be 0
	if sum != 0 {
		return Transaction{}, fmt.Errorf("%w: entries sum to %d", ErrUnbalanced, sum)
	}

	return Transaction{
		ID:          id,
		Description: desc,
		PostedAt:    now,
		Entries:     slices.Clone(entries),
	}, nil
}
