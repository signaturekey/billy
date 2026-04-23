package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestTransferServiceRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()

	accounts, from, to := newTransferTestAccounts()
	service := NewTransferService(transferTestTxManager{}, accounts, &transferTestRepository{}, &transferTestLedgerRepository{})

	_, err := service.Create(context.Background(), 1, from.ID, to.ID, 0)
	if !errors.Is(err, domainerrors.ErrInvalidAmount) {
		t.Fatalf("transfer zero error = %v, want ErrInvalidAmount", err)
	}
}

func TestTransferServiceRejectsSameAccount(t *testing.T) {
	t.Parallel()

	accounts, from, _ := newTransferTestAccounts()
	service := NewTransferService(transferTestTxManager{}, accounts, &transferTestRepository{}, &transferTestLedgerRepository{})

	_, err := service.Create(context.Background(), 1, from.ID, from.ID, 10)
	if !errors.Is(err, domainerrors.ErrSameAccountTransfer) {
		t.Fatalf("same account transfer error = %v, want ErrSameAccountTransfer", err)
	}
}

func TestTransferServiceRejectsCurrencyMismatch(t *testing.T) {
	t.Parallel()

	accounts, from, to := newTransferTestAccounts()
	to.Currency = "EUR"
	accounts.records[to.ID] = to
	service := NewTransferService(transferTestTxManager{}, accounts, &transferTestRepository{}, &transferTestLedgerRepository{})

	_, err := service.Create(context.Background(), 1, from.ID, to.ID, 10)
	if !errors.Is(err, domainerrors.ErrCurrencyMismatch) {
		t.Fatalf("currency mismatch error = %v, want ErrCurrencyMismatch", err)
	}
}

func TestTransferServiceRejectsBlockedAccount(t *testing.T) {
	t.Parallel()

	accounts, from, to := newTransferTestAccounts()
	from.Status = entity.AccountStatusBlocked
	accounts.records[from.ID] = from
	service := NewTransferService(transferTestTxManager{}, accounts, &transferTestRepository{}, &transferTestLedgerRepository{})

	_, err := service.Create(context.Background(), 1, from.ID, to.ID, 10)
	if !errors.Is(err, domainerrors.ErrAccountBlocked) {
		t.Fatalf("blocked account error = %v, want ErrAccountBlocked", err)
	}
}

func TestTransferServiceRejectsInsufficientAvailableFunds(t *testing.T) {
	t.Parallel()

	accounts, from, to := newTransferTestAccounts()
	from.ReservedAmount = 95
	accounts.records[from.ID] = from
	service := NewTransferService(transferTestTxManager{}, accounts, &transferTestRepository{}, &transferTestLedgerRepository{})

	_, err := service.Create(context.Background(), 1, from.ID, to.ID, 10)
	if !errors.Is(err, domainerrors.ErrInsufficientFunds) {
		t.Fatalf("insufficient funds error = %v, want ErrInsufficientFunds", err)
	}
}

func TestTransferServiceRejectsUnauthorizedSourceAccount(t *testing.T) {
	t.Parallel()

	accounts, from, to := newTransferTestAccounts()
	service := NewTransferService(transferTestTxManager{}, accounts, &transferTestRepository{}, &transferTestLedgerRepository{})

	_, err := service.Create(context.Background(), 99, from.ID, to.ID, 10)
	if !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("unauthorized source account error = %v, want ErrForbidden", err)
	}
}

func TestTransferServiceMovesMoneyAndWritesLedgerEntries(t *testing.T) {
	t.Parallel()

	accounts, from, to := newTransferTestAccounts()
	ledger := &transferTestLedgerRepository{}
	service := NewTransferService(transferTestTxManager{}, accounts, &transferTestRepository{}, ledger)

	transfer, err := service.Create(context.Background(), 1, from.ID, to.ID, 25)
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	if transfer.Status != entity.TransferStatusCompleted {
		t.Fatalf("transfer status = %q, want completed", transfer.Status)
	}

	updatedFrom, _ := accounts.GetByID(context.Background(), from.ID)
	updatedTo, _ := accounts.GetByID(context.Background(), to.ID)
	if updatedFrom.Balance != 75 || updatedTo.Balance != 75 {
		t.Fatalf("balances = from %d to %d, want 75/75", updatedFrom.Balance, updatedTo.Balance)
	}
	if len(ledger.events) != 2 {
		t.Fatalf("ledger entries = %d, want 2", len(ledger.events))
	}
	if ledger.events[0].Type != entity.LedgerEntryTypeTransferOut || ledger.events[1].Type != entity.LedgerEntryTypeTransferIn {
		t.Fatalf("ledger types = %q/%q, want transfer_out/transfer_in", ledger.events[0].Type, ledger.events[1].Type)
	}
}

func newTransferTestAccounts() (*transferTestAccountRepository, entity.Account, entity.Account) {
	accounts := newTransferTestAccountRepository()
	from := accounts.add(entity.Account{
		UserID:   1,
		Currency: "USD",
		Balance:  100,
		Status:   entity.AccountStatusActive,
	})
	to := accounts.add(entity.Account{
		UserID:   2,
		Currency: "USD",
		Balance:  50,
		Status:   entity.AccountStatusActive,
	})
	return accounts, from, to
}

type transferTestTxManager struct{}

func (transferTestTxManager) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	return fn(ctx, nil)
}

type transferTestAccountRepository struct {
	nextID  int64
	records map[int64]entity.Account
}

func newTransferTestAccountRepository() *transferTestAccountRepository {
	return &transferTestAccountRepository{
		nextID:  1,
		records: make(map[int64]entity.Account),
	}
}

func (repo *transferTestAccountRepository) add(account entity.Account) entity.Account {
	account.ID = repo.nextID
	repo.nextID++
	repo.records[account.ID] = account
	return account
}

func (repo *transferTestAccountRepository) Create(_ context.Context, account entity.Account) (entity.Account, error) {
	return repo.add(account), nil
}

func (repo *transferTestAccountRepository) GetByID(_ context.Context, id int64) (entity.Account, error) {
	account, ok := repo.records[id]
	if !ok {
		return entity.Account{}, domainerrors.ErrAccountNotFound
	}
	return account, nil
}

func (repo *transferTestAccountRepository) GetForUpdate(ctx context.Context, _ pgx.Tx, id int64) (entity.Account, error) {
	return repo.GetByID(ctx, id)
}

func (repo *transferTestAccountRepository) UpdateBalance(_ context.Context, _ pgx.Tx, accountID int64, balance int64) error {
	account, ok := repo.records[accountID]
	if !ok {
		return domainerrors.ErrAccountNotFound
	}
	account.Balance = balance
	repo.records[accountID] = account
	return nil
}

func (repo *transferTestAccountRepository) UpdateAmounts(
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

type transferTestRepository struct {
	nextID int64
}

func (repo *transferTestRepository) Create(_ context.Context, _ pgx.Tx, transfer entity.Transfer) (entity.Transfer, error) {
	repo.nextID++
	transfer.ID = repo.nextID
	transfer.CreatedAt = time.Now()
	return transfer, nil
}

type transferTestLedgerRepository struct {
	nextID int64
	events []entity.LedgerEntry
}

func (repo *transferTestLedgerRepository) Create(_ context.Context, _ pgx.Tx, entry entity.LedgerEntry) (entity.LedgerEntry, error) {
	repo.nextID++
	entry.ID = repo.nextID
	repo.events = append(repo.events, entry)
	return entry, nil
}

func (repo *transferTestLedgerRepository) ListByAccount(_ context.Context, accountID int64, _ int, _ int) ([]entity.LedgerEntry, error) {
	entries := make([]entity.LedgerEntry, 0)
	for _, entry := range repo.events {
		if entry.AccountID == accountID {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}
