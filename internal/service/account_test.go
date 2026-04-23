package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestAccountServiceCreateValidation(t *testing.T) {
	t.Parallel()

	accounts := newAccountTestRepository()
	service := NewAccountService(accountTestTxManager{}, accounts, &accountTestLedgerRepository{})

	account, err := service.Create(context.Background(), 10, "usd")
	require.NoError(t, err)
	assert.Equal(t, "USD", account.Currency)

	_, err = service.Create(context.Background(), 10, "US")
	require.ErrorIs(t, err, domainerrors.ErrInvalidCurrency)
}

func TestAccountServiceGetByIDRejectsOtherUserAccount(t *testing.T) {
	t.Parallel()

	accounts := newAccountTestRepository()
	account := accounts.add(entity.Account{
		UserID:   10,
		Currency: "USD",
		Status:   entity.AccountStatusActive,
	})
	service := NewAccountService(accountTestTxManager{}, accounts, &accountTestLedgerRepository{})

	_, err := service.GetByID(context.Background(), 20, account.ID)
	require.ErrorIs(t, err, domainerrors.ErrForbidden)
}

func TestAccountServiceGetBalanceUsesAvailableAmount(t *testing.T) {
	t.Parallel()

	accounts := newAccountTestRepository()
	account := accounts.add(entity.Account{
		UserID:         10,
		Currency:       "USD",
		Balance:        100,
		ReservedAmount: 30,
		Status:         entity.AccountStatusActive,
	})
	service := NewAccountService(accountTestTxManager{}, accounts, &accountTestLedgerRepository{})

	balance, err := service.GetBalance(context.Background(), 10, account.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(70), balance.AvailableAmount)
}

func TestAccountServiceTopupRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()

	accounts := newAccountTestRepository()
	account := accounts.add(entity.Account{UserID: 1, Currency: "USD", Status: entity.AccountStatusActive})
	service := NewAccountService(accountTestTxManager{}, accounts, &accountTestLedgerRepository{})

	_, err := service.TopUp(context.Background(), 1, account.ID, 0)
	require.ErrorIs(t, err, domainerrors.ErrInvalidAmount)
}

func TestAccountServiceWithdrawRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()

	accounts := newAccountTestRepository()
	account := accounts.add(entity.Account{UserID: 1, Currency: "USD", Balance: 100, Status: entity.AccountStatusActive})
	service := NewAccountService(accountTestTxManager{}, accounts, &accountTestLedgerRepository{})

	_, err := service.Withdraw(context.Background(), 1, account.ID, -1)
	require.ErrorIs(t, err, domainerrors.ErrInvalidAmount)
}

func TestAccountServiceWithdrawRejectsInsufficientAvailableFunds(t *testing.T) {
	t.Parallel()

	accounts := newAccountTestRepository()
	account := accounts.add(entity.Account{
		UserID:         1,
		Currency:       "USD",
		Balance:        100,
		ReservedAmount: 80,
		Status:         entity.AccountStatusActive,
	})
	service := NewAccountService(accountTestTxManager{}, accounts, &accountTestLedgerRepository{})

	_, err := service.Withdraw(context.Background(), 1, account.ID, 21)
	require.ErrorIs(t, err, domainerrors.ErrInsufficientFunds)
}

func TestAccountServiceWithdrawUsesAvailableBalance(t *testing.T) {
	t.Parallel()

	accounts := newAccountTestRepository()
	account := accounts.add(entity.Account{
		UserID:         1,
		Currency:       "USD",
		Balance:        100,
		ReservedAmount: 80,
		Status:         entity.AccountStatusActive,
	})
	service := NewAccountService(accountTestTxManager{}, accounts, &accountTestLedgerRepository{})

	_, err := service.Withdraw(context.Background(), 1, account.ID, 20)
	require.NoError(t, err)

	updated, err := accounts.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(80), updated.Balance)
	assert.Equal(t, int64(80), updated.ReservedAmount)
}

type accountTestTxManager struct{}

func (accountTestTxManager) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	return fn(ctx, nil)
}

type accountTestRepository struct {
	nextID  int64
	records map[int64]entity.Account
}

func newAccountTestRepository() *accountTestRepository {
	return &accountTestRepository{
		nextID:  1,
		records: make(map[int64]entity.Account),
	}
}

func (repo *accountTestRepository) add(account entity.Account) entity.Account {
	account.ID = repo.nextID
	repo.nextID++
	repo.records[account.ID] = account
	return account
}

func (repo *accountTestRepository) Create(_ context.Context, account entity.Account) (entity.Account, error) {
	for _, existing := range repo.records {
		if existing.UserID == account.UserID && existing.Currency == account.Currency {
			return entity.Account{}, domainerrors.ErrAccountAlreadyExists
		}
	}
	return repo.add(account), nil
}

func (repo *accountTestRepository) GetByID(_ context.Context, id int64) (entity.Account, error) {
	account, ok := repo.records[id]
	if !ok {
		return entity.Account{}, domainerrors.ErrAccountNotFound
	}
	return account, nil
}

func (repo *accountTestRepository) GetForUpdate(ctx context.Context, _ pgx.Tx, id int64) (entity.Account, error) {
	return repo.GetByID(ctx, id)
}

func (repo *accountTestRepository) UpdateBalance(_ context.Context, _ pgx.Tx, accountID int64, balance int64) error {
	account, ok := repo.records[accountID]
	if !ok {
		return domainerrors.ErrAccountNotFound
	}
	account.Balance = balance
	repo.records[accountID] = account
	return nil
}

func (repo *accountTestRepository) UpdateAmounts(
	_ context.Context,
	_ pgx.Tx,
	accountID int64,
	balance int64,
	reservedAmount int64,
) error {
	account, ok := repo.records[accountID]
	if !ok {
		return domainerrors.ErrAccountNotFound
	}
	account.Balance = balance
	account.ReservedAmount = reservedAmount
	repo.records[accountID] = account
	return nil
}

type accountTestLedgerRepository struct {
	nextID int64
	events []entity.LedgerEntry
}

func (repo *accountTestLedgerRepository) Create(_ context.Context, _ pgx.Tx, entry entity.LedgerEntry) (entity.LedgerEntry, error) {
	repo.nextID++
	entry.ID = repo.nextID
	entry.CreatedAt = time.Now()
	repo.events = append(repo.events, entry)
	return entry, nil
}

func (repo *accountTestLedgerRepository) ListByAccount(_ context.Context, accountID int64, _ int, _ int) ([]entity.LedgerEntry, error) {
	entries := make([]entity.LedgerEntry, 0)
	for _, entry := range repo.events {
		if entry.AccountID == accountID {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}
