package api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	defaultEntriesLimit = 50
	maxEntriesLimit     = 200
)

type balanceResponse struct {
	AccountID uuid.UUID `json:"account_id"`
	Balance   int64     `json:"balance"`
	Currency  string    `json:"currency"`
	AsOfSeq   int64     `json:"as_of_seq"`
}

type entriesResponse struct {
	Entries      []accountEntryResponse `json:"entries"`
	NextAfterSeq int64                  `json:"next_after_seq"`
}

type accountEntryResponse struct {
	ID            uuid.UUID `json:"id"`
	TransactionID uuid.UUID `json:"transaction_id"`
	Seq           int64     `json:"seq"`
	Amount        int64     `json:"amount"`
	Description   string    `json:"description"`
	PostedAt      time.Time `json:"posted_at"`
}

func (s *server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "account id must be a UUID")
		return
	}

	b, err := s.journal.GetBalance(r.Context(), id)
	if err != nil {
		s.writeErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, balanceResponse{
		AccountID: b.AccountID,
		Balance:   b.Amount,
		Currency:  string(b.Currency),
		AsOfSeq:   b.AsOfSeq,
	})
}

func (s *server) handleListEntries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "account id must be a UUID")
		return
	}

	afterSeq, err := queryInt(r, "after_seq", 0, 0, math.MaxInt64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	limit, err := queryInt(r, "limit", defaultEntriesLimit, 1, maxEntriesLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	entries, err := s.journal.ListEntries(r.Context(), id, afterSeq, int(limit))
	if err != nil {
		s.writeErr(w, r, err)
		return
	}

	resp := entriesResponse{Entries: make([]accountEntryResponse, 0, len(entries)), NextAfterSeq: afterSeq}
	for _, e := range entries {
		resp.Entries = append(resp.Entries, accountEntryResponse{
			ID:            e.EntryID,
			TransactionID: e.TransactionID,
			Seq:           e.Seq,
			Amount:        e.Amount,
			Description:   e.Description,
			PostedAt:      e.PostedAt,
		})
		resp.NextAfterSeq = e.Seq
	}
	writeJSON(w, http.StatusOK, resp)
}

// missing params fallback to default
func queryInt(r *http.Request, name string, def, lo, hi int64) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("%s must be an integer from %d to %d", name, lo, hi)
	}
	return n, nil
}
