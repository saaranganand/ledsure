package ledger

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxAccountNameLength = 200

type Currency string

const (
	CAD Currency = "CAD"
	USD Currency = "USD"
)

func ParseCurrency(s string) (Currency, error) {
	c := Currency(s)

	switch c {
	case CAD, USD:
		return c, nil
	default:
		return "", fmt.Errorf("%w: unsupported currency %q", ErrInvalidInput, s)
	}
}

type Kind string

const (
	KindCustomer Kind = "customer"
	KindSystem   Kind = "system"
)

type Account struct {
	ID            uuid.UUID
	Name          string
	Currency      Currency
	Kind          Kind
	Code          *string
	AllowNegative bool
	CreatedAt     time.Time
}

var ErrInvalidInput = errors.New("invalid input")

func NewCustomerAccount(id uuid.UUID, now time.Time, name string, currency Currency) (Account, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Account{}, fmt.Errorf("%w: name empty", ErrInvalidInput)
	}
	if utf8.RuneCountInString(name) > maxAccountNameLength {
		return Account{}, fmt.Errorf("%w: name too long", ErrInvalidInput)
	}

	if _, err := ParseCurrency(string(currency)); err != nil {
		return Account{}, err
	}

	return Account{
		ID:            id,
		Name:          name,
		Currency:      currency,
		Kind:          KindCustomer,
		Code:          nil,
		AllowNegative: false,
		CreatedAt:     now,
	}, nil
}
