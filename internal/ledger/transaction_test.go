package ledger

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAddChecked(t *testing.T) {
	tests := []struct {
		name   string
		a, b   int64
		want   int64
		wantOK bool
	}{
		{name: "positives", a: 5, b: 6, want: 11, wantOK: true},
		{name: "mixed signs", a: -5, b: 6, want: 1, wantOK: true},
		{name: "max plus zero", a: math.MaxInt64, b: 0, want: math.MaxInt64, wantOK: true},
		{name: "max plus min", a: math.MaxInt64, b: math.MinInt64, want: -1, wantOK: true},
		{name: "overflow", a: math.MaxInt64, b: 1, wantOK: false},
		{name: "underflow", a: math.MinInt64, b: -1, wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := addChecked(tc.a, tc.b)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

var (
	txID    = uuid.MustParse("00000000-0000-7000-8000-0000000000a1")
	txNow   = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	alice   = uuid.MustParse("00000000-0000-7000-8000-0000000000c1")
	bob     = uuid.MustParse("00000000-0000-7000-8000-0000000000c2")
	revenue = uuid.MustParse("00000000-0000-7000-8000-0000000000c3")
)

func entry(n byte, account uuid.UUID, amount int64) Entry {
	return Entry{ID: uuid.UUID{15: n}, AccountID: account, Amount: amount}
}

// n entries on distinct accounts
func manyEntries(n int) []Entry {
	out := make([]Entry, n)
	for i := range out {
		out[i] = Entry{ID: uuid.UUID{15: byte(i + 1)}, AccountID: uuid.UUID{14: byte(i + 1)}, Amount: 1}
	}
	return out
}

func TestNewTransaction(t *testing.T) {
	transfer := []Entry{entry(1, alice, -100), entry(2, bob, 100)}
	withFee := []Entry{entry(1, alice, -100), entry(2, bob, 98), entry(3, revenue, 2)}
	atLimit := []Entry{entry(1, alice, -maxAmount), entry(2, bob, maxAmount)}

	tests := []struct {
		name    string
		desc    string
		entries []Entry
		want    Transaction
		wantErr error
	}{
		{
			name:    "transfer",
			desc:    "rent",
			entries: transfer,
			want:    Transaction{ID: txID, Description: "rent", PostedAt: txNow, Entries: transfer},
		},
		{
			name:    "three entries with fee",
			entries: withFee,
			want:    Transaction{ID: txID, PostedAt: txNow, Entries: withFee},
		},
		{
			name:    "description trimmed",
			desc:    "  rent  ",
			entries: transfer,
			want:    Transaction{ID: txID, Description: "rent", PostedAt: txNow, Entries: transfer},
		},
		{
			name:    "amounts at the limit",
			entries: atLimit,
			want:    Transaction{ID: txID, PostedAt: txNow, Entries: atLimit},
		},
		{name: "description too long", desc: strings.Repeat("a", maxDescriptionLength+1), entries: transfer, wantErr: ErrInvalidInput},
		{name: "no entries", entries: nil, wantErr: ErrInvalidInput},
		{name: "one entry", entries: []Entry{entry(1, alice, 100)}, wantErr: ErrInvalidInput},
		{name: "too many entries", entries: manyEntries(maxEntries + 1), wantErr: ErrInvalidInput},
		{name: "zero amount", entries: []Entry{entry(1, alice, 0), entry(2, bob, 0)}, wantErr: ErrInvalidInput},
		{name: "amount above limit", entries: []Entry{entry(1, alice, -maxAmount-1), entry(2, bob, maxAmount+1)}, wantErr: ErrInvalidInput},
		{name: "only the upper bound exceeded", entries: []Entry{entry(1, alice, maxAmount+1), entry(2, bob, -maxAmount), entry(3, revenue, -1)}, wantErr: ErrInvalidInput},
		{name: "only the lower bound exceeded", entries: []Entry{entry(1, alice, -maxAmount-1), entry(2, bob, maxAmount), entry(3, revenue, 1)}, wantErr: ErrInvalidInput},
		{name: "same account twice", entries: []Entry{entry(1, alice, -50), entry(2, alice, 50)}, wantErr: ErrInvalidInput},
		{name: "unbalanced", entries: []Entry{entry(1, alice, -100), entry(2, bob, 99)}, wantErr: ErrUnbalanced},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewTransaction(txID, txNow, tc.desc, tc.entries)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNewTransactionCopiesEntries(t *testing.T) {
	in := []Entry{entry(1, alice, -100), entry(2, bob, 100)}

	tx, err := NewTransaction(txID, txNow, "", in)
	if err != nil {
		t.Fatalf("NewTransaction: %v", err)
	}

	in[0].Amount = -999999
	if tx.Entries[0].Amount != -100 {
		t.Errorf("transaction changed after the caller edited its slice: amount = %d", tx.Entries[0].Amount)
	}
}
