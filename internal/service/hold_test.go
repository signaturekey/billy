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

func TestHoldServiceCreateRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()

	accounts, account := newHoldTestAccount()
	service := NewHoldService(holdTestTxManager{}, accounts, newHoldTestRepository(), &holdTestLedgerRepository{}, time.Hour)

	if _, err := service.Create(context.Background(), 1, account.ID, 0); !errors.Is(err, domainerrors.ErrInvalidAmount) {
		t.Fatalf("create hold zero error = %v, want ErrInvalidAmount", err)
	}
}

func TestHoldServiceCreateRejectsInsufficientAvailableFunds(t *testing.T) {
	t.Parallel()

	accounts, account := newHoldTestAccount()
	account.ReservedAmount = 90
	accounts.records[account.ID] = account
	service := NewHoldService(holdTestTxManager{}, accounts, newHoldTestRepository(), &holdTestLedgerRepository{}, time.Hour)

	if _, err := service.Create(context.Background(), 1, account.ID, 11); !errors.Is(err, domainerrors.ErrInsufficientFunds) {
		t.Fatalf("create hold over available error = %v, want ErrInsufficientFunds", err)
	}
}

func TestHoldServiceCreateIncreasesReservedAmount(t *testing.T) {
	t.Parallel()

	accounts, account := newHoldTestAccount()
	service := NewHoldService(holdTestTxManager{}, accounts, newHoldTestRepository(), &holdTestLedgerRepository{}, time.Hour)

	hold, err := service.Create(context.Background(), 1, account.ID, 30)
	if err != nil {
		t.Fatalf("create hold: %v", err)
	}
	if hold.Status != entity.HoldStatusPending {
		t.Fatalf("hold status = %q, want pending", hold.Status)
	}

	updated, _ := accounts.GetByID(context.Background(), account.ID)
	if updated.ReservedAmount != 30 {
		t.Fatalf("reserved amount = %d, want 30", updated.ReservedAmount)
	}
}

func TestHoldServiceConfirmOnlyPendingHold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  entity.HoldStatus
		wantErr error
	}{
		{name: "confirmed", status: entity.HoldStatusConfirmed, wantErr: domainerrors.ErrHoldAlreadyConfirmed},
		{name: "cancelled", status: entity.HoldStatusCancelled, wantErr: domainerrors.ErrHoldAlreadyCancelled},
		{name: "expired", status: entity.HoldStatusExpired, wantErr: domainerrors.ErrHoldExpired},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accounts, account := newHoldTestAccount()
			account.ReservedAmount = 50
			accounts.records[account.ID] = account
			holds := newHoldTestRepository()
			hold := holds.add(entity.Hold{
				AccountID: account.ID,
				Amount:    50,
				Status:    tt.status,
				ExpiresAt: time.Now().Add(time.Hour),
			})
			service := NewHoldService(holdTestTxManager{}, accounts, holds, &holdTestLedgerRepository{}, time.Hour)

			_, err := service.Confirm(context.Background(), 1, hold.ID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("confirm %s hold error = %v, want %v", tt.status, err, tt.wantErr)
			}
		})
	}
}

func TestHoldServiceCancelOnlyPendingHold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  entity.HoldStatus
		wantErr error
	}{
		{name: "confirmed", status: entity.HoldStatusConfirmed, wantErr: domainerrors.ErrHoldAlreadyConfirmed},
		{name: "cancelled", status: entity.HoldStatusCancelled, wantErr: domainerrors.ErrHoldAlreadyCancelled},
		{name: "expired", status: entity.HoldStatusExpired, wantErr: domainerrors.ErrHoldExpired},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accounts, account := newHoldTestAccount()
			account.ReservedAmount = 50
			accounts.records[account.ID] = account
			holds := newHoldTestRepository()
			hold := holds.add(entity.Hold{
				AccountID: account.ID,
				Amount:    50,
				Status:    tt.status,
				ExpiresAt: time.Now().Add(time.Hour),
			})
			service := NewHoldService(holdTestTxManager{}, accounts, holds, &holdTestLedgerRepository{}, time.Hour)

			_, err := service.Cancel(context.Background(), 1, hold.ID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("cancel %s hold error = %v, want %v", tt.status, err, tt.wantErr)
			}
		})
	}
}

func TestHoldServiceConfirmRejectsExpiredPendingHold(t *testing.T) {
	t.Parallel()

	accounts, account := newHoldTestAccount()
	account.ReservedAmount = 50
	accounts.records[account.ID] = account
	holds := newHoldTestRepository()
	hold := holds.add(entity.Hold{
		AccountID: account.ID,
		Amount:    50,
		Status:    entity.HoldStatusPending,
		ExpiresAt: time.Now().Add(-time.Hour),
	})
	service := NewHoldService(holdTestTxManager{}, accounts, holds, &holdTestLedgerRepository{}, time.Hour)

	if _, err := service.Confirm(context.Background(), 1, hold.ID); !errors.Is(err, domainerrors.ErrHoldExpired) {
		t.Fatalf("confirm expired pending hold error = %v, want ErrHoldExpired", err)
	}
}

func TestHoldServiceConfirmChargesAndReleasesReservedAmount(t *testing.T) {
	t.Parallel()

	accounts, account := newHoldTestAccount()
	account.ReservedAmount = 50
	accounts.records[account.ID] = account
	holds := newHoldTestRepository()
	hold := holds.add(entity.Hold{
		AccountID: account.ID,
		Amount:    50,
		Status:    entity.HoldStatusPending,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	service := NewHoldService(holdTestTxManager{}, accounts, holds, &holdTestLedgerRepository{}, time.Hour)

	updatedHold, err := service.Confirm(context.Background(), 1, hold.ID)
	if err != nil {
		t.Fatalf("confirm hold: %v", err)
	}
	if updatedHold.Status != entity.HoldStatusConfirmed {
		t.Fatalf("hold status = %q, want confirmed", updatedHold.Status)
	}

	updatedAccount, _ := accounts.GetByID(context.Background(), account.ID)
	if updatedAccount.Balance != 50 || updatedAccount.ReservedAmount != 0 {
		t.Fatalf("amounts after confirm = balance %d reserved %d, want 50/0", updatedAccount.Balance, updatedAccount.ReservedAmount)
	}
}

func TestHoldServiceCancelReleasesReservedAmount(t *testing.T) {
	t.Parallel()

	accounts, account := newHoldTestAccount()
	account.ReservedAmount = 50
	accounts.records[account.ID] = account
	holds := newHoldTestRepository()
	hold := holds.add(entity.Hold{
		AccountID: account.ID,
		Amount:    50,
		Status:    entity.HoldStatusPending,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	service := NewHoldService(holdTestTxManager{}, accounts, holds, &holdTestLedgerRepository{}, time.Hour)

	updatedHold, err := service.Cancel(context.Background(), 1, hold.ID)
	if err != nil {
		t.Fatalf("cancel hold: %v", err)
	}
	if updatedHold.Status != entity.HoldStatusCancelled {
		t.Fatalf("hold status = %q, want cancelled", updatedHold.Status)
	}

	updatedAccount, _ := accounts.GetByID(context.Background(), account.ID)
	if updatedAccount.Balance != 100 || updatedAccount.ReservedAmount != 0 {
		t.Fatalf("amounts after cancel = balance %d reserved %d, want 100/0", updatedAccount.Balance, updatedAccount.ReservedAmount)
	}
}

func TestHoldServiceExpirePendingReleasesOnlyExpiredPendingHolds(t *testing.T) {
	t.Parallel()

	accounts, account := newHoldTestAccount()
	account.ReservedAmount = 70
	accounts.records[account.ID] = account
	holds := newHoldTestRepository()
	expired := holds.add(entity.Hold{
		AccountID: account.ID,
		Amount:    40,
		Status:    entity.HoldStatusPending,
		ExpiresAt: time.Now().Add(-time.Hour),
	})
	holds.add(entity.Hold{
		AccountID: account.ID,
		Amount:    30,
		Status:    entity.HoldStatusPending,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	holds.add(entity.Hold{
		AccountID: account.ID,
		Amount:    10,
		Status:    entity.HoldStatusCancelled,
		ExpiresAt: time.Now().Add(-time.Hour),
	})
	service := NewHoldService(holdTestTxManager{}, accounts, holds, &holdTestLedgerRepository{}, time.Hour)

	count, err := service.ExpirePending(context.Background(), 10)
	if err != nil {
		t.Fatalf("expire pending: %v", err)
	}
	if count != 1 {
		t.Fatalf("expired count = %d, want 1", count)
	}

	updatedHold := holds.records[expired.ID]
	if updatedHold.Status != entity.HoldStatusExpired {
		t.Fatalf("expired hold status = %q, want expired", updatedHold.Status)
	}
	updatedAccount, _ := accounts.GetByID(context.Background(), account.ID)
	if updatedAccount.ReservedAmount != 30 {
		t.Fatalf("reserved amount after expiration = %d, want 30", updatedAccount.ReservedAmount)
	}

	secondCount, err := service.ExpirePending(context.Background(), 10)
	if err != nil {
		t.Fatalf("expire pending second run: %v", err)
	}
	if secondCount != 0 {
		t.Fatalf("second expired count = %d, want 0", secondCount)
	}
}

func newHoldTestAccount() (*holdTestAccountRepository, entity.Account) {
	accounts := newHoldTestAccountRepository()
	account := accounts.add(entity.Account{
		UserID:   1,
		Currency: "USD",
		Balance:  100,
		Status:   entity.AccountStatusActive,
	})
	return accounts, account
}

type holdTestTxManager struct{}

func (holdTestTxManager) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	return fn(ctx, nil)
}

type holdTestAccountRepository struct {
	nextID  int64
	records map[int64]entity.Account
}

func newHoldTestAccountRepository() *holdTestAccountRepository {
	return &holdTestAccountRepository{
		nextID:  1,
		records: make(map[int64]entity.Account),
	}
}

func (repo *holdTestAccountRepository) add(account entity.Account) entity.Account {
	account.ID = repo.nextID
	repo.nextID++
	repo.records[account.ID] = account
	return account
}

func (repo *holdTestAccountRepository) Create(_ context.Context, account entity.Account) (entity.Account, error) {
	return repo.add(account), nil
}

func (repo *holdTestAccountRepository) GetByID(_ context.Context, id int64) (entity.Account, error) {
	account, ok := repo.records[id]
	if !ok {
		return entity.Account{}, domainerrors.ErrAccountNotFound
	}
	return account, nil
}

func (repo *holdTestAccountRepository) GetForUpdate(ctx context.Context, _ pgx.Tx, id int64) (entity.Account, error) {
	return repo.GetByID(ctx, id)
}

func (repo *holdTestAccountRepository) UpdateBalance(_ context.Context, _ pgx.Tx, accountID int64, balance int64) error {
	account, ok := repo.records[accountID]
	if !ok {
		return domainerrors.ErrAccountNotFound
	}
	account.Balance = balance
	repo.records[accountID] = account
	return nil
}

func (repo *holdTestAccountRepository) UpdateAmounts(
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

type holdTestRepository struct {
	nextID  int64
	records map[int64]entity.Hold
}

func newHoldTestRepository() *holdTestRepository {
	return &holdTestRepository{
		nextID:  1,
		records: make(map[int64]entity.Hold),
	}
}

func (repo *holdTestRepository) add(hold entity.Hold) entity.Hold {
	hold.ID = repo.nextID
	repo.nextID++
	repo.records[hold.ID] = hold
	return hold
}

func (repo *holdTestRepository) ListExpiredPending(_ context.Context, now time.Time, limit int) ([]entity.Hold, error) {
	holds := make([]entity.Hold, 0)
	for _, hold := range repo.records {
		if len(holds) == limit {
			break
		}
		if hold.Status == entity.HoldStatusPending && hold.ExpiresAt.Before(now) {
			holds = append(holds, hold)
		}
	}
	return holds, nil
}

func (repo *holdTestRepository) Create(_ context.Context, _ pgx.Tx, hold entity.Hold) (entity.Hold, error) {
	return repo.add(hold), nil
}

func (repo *holdTestRepository) GetByIDForUpdate(_ context.Context, _ pgx.Tx, id int64) (entity.Hold, error) {
	hold, ok := repo.records[id]
	if !ok {
		return entity.Hold{}, domainerrors.ErrHoldNotFound
	}
	return hold, nil
}

func (repo *holdTestRepository) UpdateStatus(_ context.Context, _ pgx.Tx, id int64, status entity.HoldStatus) (entity.Hold, error) {
	hold, ok := repo.records[id]
	if !ok {
		return entity.Hold{}, domainerrors.ErrHoldNotFound
	}
	hold.Status = status
	repo.records[id] = hold
	return hold, nil
}

type holdTestLedgerRepository struct {
	nextID int64
	events []entity.LedgerEntry
}

func (repo *holdTestLedgerRepository) Create(_ context.Context, _ pgx.Tx, entry entity.LedgerEntry) (entity.LedgerEntry, error) {
	repo.nextID++
	entry.ID = repo.nextID
	repo.events = append(repo.events, entry)
	return entry, nil
}

func (repo *holdTestLedgerRepository) ListByAccount(_ context.Context, accountID int64, _ int, _ int) ([]entity.LedgerEntry, error) {
	entries := make([]entity.LedgerEntry, 0)
	for _, entry := range repo.events {
		if entry.AccountID == accountID {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}
