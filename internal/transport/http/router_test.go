package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
	"github.com/signaturekey/billy/internal/service"
	transporthandler "github.com/signaturekey/billy/internal/transport/http/handler"
)

func TestRouterAuthContract(t *testing.T) {
	t.Parallel()

	server := newHandlerTestServer(&handlerTestServices{})

	tests := []struct {
		name   string
		userID string
	}{
		{name: "missing user id"},
		{name: "invalid user id", userID: "not-int"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := server.do(t, stdhttp.MethodGet, "/api/v1/accounts/1", tt.userID, "", nil)
			assert.Equal(t, stdhttp.StatusUnauthorized, response.Code)
		})
	}
}

func TestRouterRequestValidationContract(t *testing.T) {
	t.Parallel()

	server := newHandlerTestServer(&handlerTestServices{})

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "invalid json",
			method: stdhttp.MethodPost,
			path:   "/api/v1/accounts",
			body:   []byte(`{`),
		},
		{
			name:   "invalid path id",
			method: stdhttp.MethodGet,
			path:   "/api/v1/accounts/not-int",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := server.do(t, tt.method, tt.path, "10", "", tt.body)
			assert.Equal(t, stdhttp.StatusBadRequest, response.Code)
		})
	}
}

func TestRouterDomainErrorMappingContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		services      *handlerTestServices
		method        string
		path          string
		body          []byte
		idempotency   string
		wantStatus    int
		wantErrorText string
	}{
		{
			name:          "not found maps to 404",
			services:      &handlerTestServices{account: &fakeAccountService{getErr: domainerrors.ErrAccountNotFound}},
			method:        stdhttp.MethodGet,
			path:          "/api/v1/accounts/1",
			wantStatus:    stdhttp.StatusNotFound,
			wantErrorText: "account not found",
		},
		{
			name:          "forbidden maps to 403",
			services:      &handlerTestServices{account: &fakeAccountService{getErr: domainerrors.ErrForbidden}},
			method:        stdhttp.MethodGet,
			path:          "/api/v1/accounts/1",
			wantStatus:    stdhttp.StatusForbidden,
			wantErrorText: "forbidden",
		},
		{
			name:          "conflict maps to 409",
			services:      &handlerTestServices{account: &fakeAccountService{createErr: domainerrors.ErrAccountAlreadyExists}},
			method:        stdhttp.MethodPost,
			path:          "/api/v1/accounts",
			body:          []byte(`{"currency":"USD"}`),
			wantStatus:    stdhttp.StatusConflict,
			wantErrorText: "account already exists",
		},
		{
			name: "insufficient funds maps to 422",
			services: &handlerTestServices{
				account: &fakeAccountService{withdrawErr: domainerrors.ErrInsufficientFunds},
			},
			method:        stdhttp.MethodPost,
			path:          "/api/v1/accounts/1/withdrawals",
			body:          []byte(`{"amount":100}`),
			idempotency:   "withdraw-key",
			wantStatus:    stdhttp.StatusUnprocessableEntity,
			wantErrorText: "insufficient funds",
		},
		{
			name:          "internal error maps to 500",
			services:      &handlerTestServices{account: &fakeAccountService{getErr: errors.New("database down")}},
			method:        stdhttp.MethodGet,
			path:          "/api/v1/accounts/1",
			wantStatus:    stdhttp.StatusInternalServerError,
			wantErrorText: "internal server error",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newHandlerTestServer(tt.services)
			response := server.do(t, tt.method, tt.path, "10", tt.idempotency, tt.body)

			assert.Equal(t, tt.wantStatus, response.Code)
			assert.JSONEq(t, `{"error":`+quoteJSON(t, tt.wantErrorText)+`}`, response.Body.String())
		})
	}
}

func TestRouterSuccessStatusContract(t *testing.T) {
	t.Parallel()

	server := newHandlerTestServer(&handlerTestServices{})

	tests := []struct {
		name        string
		method      string
		path        string
		body        []byte
		idempotency string
		wantStatus  int
	}{
		{
			name:       "create returns 201",
			method:     stdhttp.MethodPost,
			path:       "/api/v1/accounts",
			body:       []byte(`{"currency":"USD"}`),
			wantStatus: stdhttp.StatusCreated,
		},
		{
			name:       "get returns 200",
			method:     stdhttp.MethodGet,
			path:       "/api/v1/accounts/1",
			wantStatus: stdhttp.StatusOK,
		},
		{
			name:       "list returns 200",
			method:     stdhttp.MethodGet,
			path:       "/api/v1/accounts/1/operations",
			wantStatus: stdhttp.StatusOK,
		},
		{
			name:        "action returns 200",
			method:      stdhttp.MethodPost,
			path:        "/api/v1/holds/1/confirm",
			idempotency: "confirm-key",
			wantStatus:  stdhttp.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := server.do(t, tt.method, tt.path, "10", tt.idempotency, tt.body)
			assert.Equal(t, tt.wantStatus, response.Code)
		})
	}
}

type handlerTestServer struct {
	handler stdhttp.Handler
}

func newHandlerTestServer(services *handlerTestServices) *handlerTestServer {
	gin.SetMode(gin.TestMode)

	if services.account == nil {
		services.account = &fakeAccountService{}
	}
	if services.transfer == nil {
		services.transfer = &fakeTransferService{}
	}
	if services.hold == nil {
		services.hold = &fakeHoldService{}
	}
	idempotency := passthroughIdempotency{}

	router := NewRouter(
		transporthandler.NewAccountHandler(services.account, idempotency),
		transporthandler.NewTransferHandler(services.transfer, idempotency),
		transporthandler.NewHoldHandler(services.hold, idempotency),
		nil,
	)

	return &handlerTestServer{handler: router.Mount()}
}

func (server *handlerTestServer) do(
	t *testing.T,
	method string,
	path string,
	userID string,
	idempotencyKey string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		request.Header.Set("X-User-ID", userID)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}

	response := httptest.NewRecorder()
	server.handler.ServeHTTP(response, request)
	return response
}

func quoteJSON(t *testing.T, value string) string {
	t.Helper()

	body, err := json.Marshal(value)
	require.NoError(t, err)
	return string(body)
}

type handlerTestServices struct {
	account  *fakeAccountService
	transfer *fakeTransferService
	hold     *fakeHoldService
}

type passthroughIdempotency struct{}

func (passthroughIdempotency) Execute(
	ctx context.Context,
	_ int64,
	_ string,
	_ string,
	_ string,
	mutate service.IdempotentMutation,
) (service.IdempotencyResult, error) {
	statusCode, payload, err := mutate(ctx, nil)
	if err != nil {
		return service.IdempotencyResult{}, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return service.IdempotencyResult{}, err
	}

	return service.IdempotencyResult{StatusCode: statusCode, Body: body}, nil
}

type fakeAccountService struct {
	createErr   error
	getErr      error
	balanceErr  error
	topupErr    error
	withdrawErr error
	listErr     error
}

func (service *fakeAccountService) Create(context.Context, int64, string) (entity.Account, error) {
	if service.createErr != nil {
		return entity.Account{}, service.createErr
	}
	return entity.Account{ID: 1, Currency: "USD", Status: entity.AccountStatusActive}, nil
}

func (service *fakeAccountService) GetByID(context.Context, int64, int64) (entity.Account, error) {
	if service.getErr != nil {
		return entity.Account{}, service.getErr
	}
	return entity.Account{ID: 1, Currency: "USD", Status: entity.AccountStatusActive}, nil
}

func (service *fakeAccountService) GetBalance(context.Context, int64, int64) (entity.AccountBalance, error) {
	if service.balanceErr != nil {
		return entity.AccountBalance{}, service.balanceErr
	}
	return entity.AccountBalance{AccountID: 1, Balance: 100, AvailableAmount: 100, Currency: "USD"}, nil
}

func (service *fakeAccountService) TopUp(context.Context, int64, int64, int64) (entity.LedgerEntry, error) {
	return entity.LedgerEntry{}, nil
}

func (service *fakeAccountService) TopUpInTx(context.Context, pgx.Tx, int64, int64, int64) (entity.LedgerEntry, error) {
	if service.topupErr != nil {
		return entity.LedgerEntry{}, service.topupErr
	}
	return testLedgerEntry(entity.LedgerEntryTypeTopup), nil
}

func (service *fakeAccountService) Withdraw(context.Context, int64, int64, int64) (entity.LedgerEntry, error) {
	return entity.LedgerEntry{}, nil
}

func (service *fakeAccountService) WithdrawInTx(context.Context, pgx.Tx, int64, int64, int64) (entity.LedgerEntry, error) {
	if service.withdrawErr != nil {
		return entity.LedgerEntry{}, service.withdrawErr
	}
	return testLedgerEntry(entity.LedgerEntryTypeWithdrawal), nil
}

func (service *fakeAccountService) ListOperations(context.Context, int64, int64, int, int) ([]entity.LedgerEntry, error) {
	if service.listErr != nil {
		return nil, service.listErr
	}
	return []entity.LedgerEntry{testLedgerEntry(entity.LedgerEntryTypeTopup)}, nil
}

type fakeTransferService struct {
	createErr error
}

func (service *fakeTransferService) Create(context.Context, int64, int64, int64, int64) (entity.Transfer, error) {
	return entity.Transfer{}, nil
}

func (service *fakeTransferService) CreateInTx(context.Context, pgx.Tx, int64, int64, int64, int64) (entity.Transfer, error) {
	if service.createErr != nil {
		return entity.Transfer{}, service.createErr
	}
	return entity.Transfer{
		ID:            1,
		FromAccountID: 1,
		ToAccountID:   2,
		Amount:        10,
		Status:        entity.TransferStatusCompleted,
		CreatedAt:     time.Now(),
	}, nil
}

type fakeHoldService struct {
	createErr  error
	confirmErr error
	cancelErr  error
}

func (service *fakeHoldService) Create(context.Context, int64, int64, int64) (entity.Hold, error) {
	return entity.Hold{}, nil
}

func (service *fakeHoldService) CreateInTx(context.Context, pgx.Tx, int64, int64, int64) (entity.Hold, error) {
	if service.createErr != nil {
		return entity.Hold{}, service.createErr
	}
	return testHold(entity.HoldStatusPending), nil
}

func (service *fakeHoldService) Confirm(context.Context, int64, int64) (entity.Hold, error) {
	return entity.Hold{}, nil
}

func (service *fakeHoldService) ConfirmInTx(context.Context, pgx.Tx, int64, int64) (entity.Hold, error) {
	if service.confirmErr != nil {
		return entity.Hold{}, service.confirmErr
	}
	return testHold(entity.HoldStatusConfirmed), nil
}

func (service *fakeHoldService) Cancel(context.Context, int64, int64) (entity.Hold, error) {
	return entity.Hold{}, nil
}

func (service *fakeHoldService) CancelInTx(context.Context, pgx.Tx, int64, int64) (entity.Hold, error) {
	if service.cancelErr != nil {
		return entity.Hold{}, service.cancelErr
	}
	return testHold(entity.HoldStatusCancelled), nil
}

func testLedgerEntry(entryType entity.LedgerEntryType) entity.LedgerEntry {
	return entity.LedgerEntry{
		ID:            1,
		AccountID:     1,
		Type:          entryType,
		Amount:        10,
		Currency:      "USD",
		BalanceBefore: 100,
		BalanceAfter:  110,
		CreatedAt:     time.Now(),
	}
}

func testHold(status entity.HoldStatus) entity.Hold {
	return entity.Hold{
		ID:        1,
		AccountID: 1,
		Amount:    10,
		Status:    status,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
