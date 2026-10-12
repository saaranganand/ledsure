package ledger

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseCurrency(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Currency
		wantErr bool
	}{
		{name: "cad", input: "CAD", want: CAD},
		{name: "usd", input: "USD", want: USD},
		{name: "lowercase", input: "cad", wantErr: true},
		{name: "unsupported", input: "EUR", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "trailing space", input: "CAD ", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCurrency(tc.input)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("err = %v, want ErrInvalidInput", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewCustomerAccount(t *testing.T) {
	id := uuid.MustParse("00000000-0000-7000-8000-0000000000aa")
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name     string
		input    string
		currency Currency
		want     Account
		wantErr  bool
	}{
		{name: "valid", input: "John", currency: CAD, want: Account{ID: id, Name: "John", Currency: CAD, Kind: KindCustomer, CreatedAt: now}},
		{name: "lead and trail spaces", input: " alice ", currency: CAD, want: Account{ID: id, Name: "alice", Currency: CAD, Kind: KindCustomer, CreatedAt: now}},
		{name: "empty name", input: "", currency: CAD, wantErr: true},
		{name: "space only", input: " ", currency: CAD, wantErr: true},
		{name: "200 chars", input: strings.Repeat("a", 200), currency: CAD, want: Account{ID: id, Name: strings.Repeat("a", 200), Currency: CAD, Kind: KindCustomer, CreatedAt: now}},
		{name: "201 chars", input: strings.Repeat("a", 201), currency: CAD, wantErr: true},
		{name: "200 multibyte chars", input: strings.Repeat("é", 200), currency: CAD, want: Account{ID: id, Name: strings.Repeat("é", 200), Currency: CAD, Kind: KindCustomer, CreatedAt: now}},
		{name: "currency EUR", input: "John", currency: Currency("EUR"), wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewCustomerAccount(id, now, tc.input, tc.currency)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("err = %v, want ErrInvalidInput", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
