package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/saaranganand/ledsure/internal/ledger"
)

const maxRequestBodyBytes = 1 << 20

type createAccountRequest struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

type accountResponse struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Currency      string    `json:"currency"`
	Kind          string    `json:"kind"`
	Code          *string   `json:"code,omitempty"`
	AllowNegative bool      `json:"allow_negative"`
	CreatedAt     time.Time `json:"created_at"`
}

func toAccountResponse(a ledger.Account) accountResponse {
	return accountResponse{
		ID:            a.ID,
		Name:          a.Name,
		Currency:      string(a.Currency),
		Kind:          string(a.Kind),
		Code:          a.Code,
		AllowNegative: a.AllowNegative,
		CreatedAt:     a.CreatedAt,
	}
}

func (s *server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	currency, err := ledger.ParseCurrency(req.Currency)
	if err != nil {
		s.writeErr(w, r, err)
		return
	}

	account, err := ledger.NewCustomerAccount(s.ids.New(), s.clock.Now(), req.Name, currency)
	if err != nil {
		s.writeErr(w, r, err)
		return
	}

	if err := s.accounts.CreateAccount(r.Context(), account); err != nil {
		s.writeErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toAccountResponse(account))
}

// rejects unknown fields
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("malformed JSON body: %w", err)
	}
	if dec.More() {
		return fmt.Errorf("body must contain a single JSON object")
	}
	return nil
}
