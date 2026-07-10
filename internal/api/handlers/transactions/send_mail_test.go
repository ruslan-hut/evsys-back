package transactions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"

	"github.com/go-chi/chi/v5"
)

// stubTransactions implements the Transactions interface; only SendTransactionMail
// is exercised here.
type stubTransactions struct {
	gotId    int
	gotEmail string
	gotUser  string
	calls    int
	err      error
}

func (s *stubTransactions) GetActiveTransactions(context.Context, string) (any, error) {
	return nil, nil
}
func (s *stubTransactions) GetTransactions(context.Context, string, string) (any, error) {
	return nil, nil
}
func (s *stubTransactions) GetFilteredTransactions(context.Context, *entity.User, *entity.TransactionFilter) (any, error) {
	return nil, nil
}
func (s *stubTransactions) GetTransaction(context.Context, string, int, int) (any, error) {
	return nil, nil
}
func (s *stubTransactions) GetRecentChargePoints(context.Context, string) (any, error) {
	return nil, nil
}

func (s *stubTransactions) SendTransactionMail(_ context.Context, author *entity.User, id int, to string) error {
	s.calls++
	s.gotId = id
	s.gotEmail = to
	s.gotUser = author.Username
	return s.err
}

// serve routes the request through a real chi router so {id} is populated the
// same way it is in production.
func serve(t *testing.T, h *stubTransactions, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	r := chi.NewRouter()
	r.Post("/transactions/info/{id}/email", SendMail(log, h))

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req = req.WithContext(cont.PutUser(req.Context(), &entity.User{
		Username: "driver", UserId: "u-1", AccessLevel: 1,
	}))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestSendMailHandler(t *testing.T) {
	t.Run("forwards id, email and caller", func(t *testing.T) {
		h := &stubTransactions{}
		rec := serve(t, h, "/transactions/info/4207/email", `{"email":"me@example.com"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
		}
		if h.calls != 1 {
			t.Fatalf("calls = %d, want 1", h.calls)
		}
		if h.gotId != 4207 || h.gotEmail != "me@example.com" || h.gotUser != "driver" {
			t.Errorf("got id=%d email=%q user=%q", h.gotId, h.gotEmail, h.gotUser)
		}

		var payload map[string]any
		body, _ := io.ReadAll(rec.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("bad json: %v (%s)", err, body)
		}
		if payload["success"] != true {
			t.Errorf("payload = %v, want success:true", payload)
		}
	})

	t.Run("non numeric id is rejected before the core call", func(t *testing.T) {
		h := &stubTransactions{}
		rec := serve(t, h, "/transactions/info/abc/email", `{"email":"me@example.com"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if h.calls != 0 {
			t.Errorf("core must not be called on a bad id")
		}
	})

	t.Run("malformed body is rejected", func(t *testing.T) {
		h := &stubTransactions{}
		rec := serve(t, h, "/transactions/info/1/email", `{`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if h.calls != 0 {
			t.Errorf("core must not be called on a bad body")
		}
	})

	t.Run("not found error maps to 404", func(t *testing.T) {
		h := &stubTransactions{err: fmt.Errorf("transaction %w", entity.ErrNotFound)}
		rec := serve(t, h, "/transactions/info/9/email", `{"email":"me@example.com"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("access denial maps to 400", func(t *testing.T) {
		h := &stubTransactions{err: fmt.Errorf("access denied: insufficient permissions")}
		rec := serve(t, h, "/transactions/info/9/email", `{"email":"me@example.com"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}
