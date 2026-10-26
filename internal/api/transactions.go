package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/saaranganand/ledsure/internal/ledger"
)

type postTransactionRequest struct {
	Description string         `json:"description"`
	Entries     []entryRequest `json:"entries"`
}

type entryRequest struct {
	AccountID uuid.UUID `json:"account_id"`
	Amount    int64     `json:"amount"`
}

type transactionResponse struct {
	ID          uuid.UUID       `json:"id"`
	Seq         int64           `json:"seq"`
	Description string          `json:"description"`
	PostedAt    time.Time       `json:"posted_at"`
	Entries     []entryResponse `json:"entries"`
}

type entryResponse struct {
	ID        uuid.UUID `json:"id"`
	AccountID uuid.UUID `json:"account_id"`
	Amount    int64     `json:"amount"`
}

func toTransactionResponse(tx ledger.Transaction) transactionResponse {
	entries := make([]entryResponse, 0, len(tx.Entries))
	for _, e := range tx.Entries {
		entries = append(entries, entryResponse{ID: e.ID, AccountID: e.AccountID, Amount: e.Amount})
	}
	return transactionResponse{
		ID:          tx.ID,
		Seq:         tx.Seq,
		Description: tx.Description,
		PostedAt:    tx.PostedAt,
		Entries:     entries,
	}
}

func (s *server) handlePostTransaction(w http.ResponseWriter, r *http.Request) {
	var req postTransactionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	txID := s.ids.New()
	entries := make([]ledger.Entry, 0, len(req.Entries))
	for _, e := range req.Entries {
		entries = append(entries, ledger.Entry{ID: s.ids.New(), AccountID: e.AccountID, Amount: e.Amount})
	}

	tx, err := ledger.NewTransaction(txID, s.clock.Now(), req.Description, entries)
	if err != nil {
		s.writeErr(w, r, err)
		return
	}

	posted, err := s.journal.PostTransaction(r.Context(), tx)
	if err != nil {
		s.writeErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toTransactionResponse(posted))
}

func (s *server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "transaction id must be a UUID")
		return
	}

	tx, err := s.journal.GetTransaction(r.Context(), id)
	if err != nil {
		s.writeErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toTransactionResponse(tx))
}
