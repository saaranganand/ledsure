package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/saaranganand/ledsure/internal/ledger"
	"github.com/saaranganand/ledsure/internal/platform"
	"github.com/saaranganand/ledsure/internal/store"
)

type AccountStore interface {
	CreateAccount(ctx context.Context, a ledger.Account) error
	GetAccount(ctx context.Context, id uuid.UUID) (ledger.Account, error)
	ListSystemAccounts(ctx context.Context) ([]ledger.Account, error)
}

type JournalStore interface {
	PostTransaction(ctx context.Context, tx ledger.Transaction) (ledger.Transaction, error)
	GetTransaction(ctx context.Context, id uuid.UUID) (ledger.Transaction, error)
	GetBalance(ctx context.Context, accountID uuid.UUID) (store.Balance, error)
	ListEntries(ctx context.Context, accountID uuid.UUID, afterSeq int64, limit int) ([]store.AccountEntry, error)
}

type Deps struct {
	Logger   *slog.Logger
	Accounts AccountStore
	Clock    platform.Clock
	IDs      platform.IDGenerator
	Journal  JournalStore
}

type server struct {
	logger   *slog.Logger
	accounts AccountStore
	clock    platform.Clock
	ids      platform.IDGenerator
	journal  JournalStore
}

func NewRouter(d Deps) http.Handler {
	s := &server{logger: d.Logger, accounts: d.Accounts, clock: d.Clock, ids: d.IDs, journal: d.Journal}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("POST /v1/accounts", s.handleCreateAccount)
	mux.HandleFunc("GET /v1/accounts/{id}", s.handleGetAccount)
	mux.HandleFunc("GET /v1/system-accounts", s.handleListSystemAccounts)
	mux.HandleFunc("POST /v1/transactions", s.handlePostTransaction)
	mux.HandleFunc("GET /v1/transactions/{id}", s.handleGetTransaction)


	return withLogging(d.Logger, mux)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		logger.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
		)
	})
}
