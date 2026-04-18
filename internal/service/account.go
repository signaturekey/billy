package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

type TxManager interface {
	WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error
}

type AccountRepository interface {
	Create(ctx context.Context, account entity.Account) (entity.Account, error)
	GetByID(ctx context.Context, id int64) (entity.Account, error)
	GetForUpdate(ctx context.Context, tx pgx.Tx, id int64) (entity.Account, error)
	UpdateBalance(ctx context.Context, tx pgx.Tx, accountID int64, balance int64) error
}

type LedgerRepository interface {
	Create(ctx context.Context, tx pgx.Tx, entry entity.LedgerEntry) (entity.LedgerEntry, error)
	ListByAccount(ctx context.Context, accountID int64, limit int, offset int) ([]entity.LedgerEntry, error)
}

type accountService struct {
	txManager  TxManager
	accounts   AccountRepository
	operations LedgerRepository
}

func NewAccountService(
	txManager TxManager,
	accounts AccountRepository,
	operations LedgerRepository,
) *accountService {
	return &accountService{
		txManager:  txManager,
		accounts:   accounts,
		operations: operations,
	}
}

func (service *accountService) Create(ctx context.Context, userID int64, currency string) (entity.Account, error) {
	normalizedCurrency := normalizeCurrency(currency)
	if !isValidCurrency(normalizedCurrency) {
		return entity.Account{}, domainerrors.ErrInvalidCurrency
	}

	account := entity.Account{
		UserID:         userID,
		Currency:       normalizedCurrency,
		Balance:        0,
		ReservedAmount: 0,
		Status:         entity.AccountStatusActive,
	}

	createdAccount, err := service.accounts.Create(ctx, account)
	if err != nil {
		return entity.Account{}, err
	}

	if err := validateAccountAmounts(createdAccount.Balance, createdAccount.ReservedAmount); err != nil {
		return entity.Account{}, err
	}

	return createdAccount, nil
}

func (service *accountService) GetByID(ctx context.Context, userID int64, accountID int64) (entity.Account, error) {
	account, err := service.accounts.GetByID(ctx, accountID)
	if err != nil {
		return entity.Account{}, err
	}

	if account.UserID != userID {
		return entity.Account{}, domainerrors.ErrForbidden
	}

	if err := validateAccountAmounts(account.Balance, account.ReservedAmount); err != nil {
		return entity.Account{}, err
	}

	return account, nil
}

func (service *accountService) GetBalance(
	ctx context.Context,
	userID int64,
	accountID int64,
) (entity.AccountBalance, error) {
	account, err := service.GetByID(ctx, userID, accountID)
	if err != nil {
		return entity.AccountBalance{}, err
	}

	return entity.AccountBalance{
		AccountID:       account.ID,
		Balance:         account.Balance,
		ReservedAmount:  account.ReservedAmount,
		AvailableAmount: account.Balance - account.ReservedAmount,
		Currency:        account.Currency,
	}, nil
}

func (service *accountService) TopUp(
	ctx context.Context,
	userID int64,
	accountID int64,
	amount int64,
) (entity.LedgerEntry, error) {
	if amount <= 0 {
		return entity.LedgerEntry{}, domainerrors.ErrInvalidAmount
	}

	var entry entity.LedgerEntry
	err := service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		account, err := service.accounts.GetForUpdate(ctx, tx, accountID)
		if err != nil {
			return err
		}

		if account.UserID != userID {
			return domainerrors.ErrForbidden
		}

		if err := validateAccountAmounts(account.Balance, account.ReservedAmount); err != nil {
			return err
		}

		balanceBefore := account.Balance
		balanceAfter := balanceBefore + amount

		if err := service.accounts.UpdateBalance(ctx, tx, account.ID, balanceAfter); err != nil {
			return err
		}

		entry, err = service.operations.Create(ctx, tx, entity.LedgerEntry{
			AccountID:     account.ID,
			Type:          entity.LedgerEntryTypeTopup,
			Amount:        amount,
			Currency:      account.Currency,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
		})
		return err
	})
	if err != nil {
		return entity.LedgerEntry{}, err
	}

	return entry, nil
}

func (service *accountService) Withdraw(
	ctx context.Context,
	userID int64,
	accountID int64,
	amount int64,
) (entity.LedgerEntry, error) {
	if amount <= 0 {
		return entity.LedgerEntry{}, domainerrors.ErrInvalidAmount
	}

	var entry entity.LedgerEntry
	err := service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		account, err := service.accounts.GetForUpdate(ctx, tx, accountID)
		if err != nil {
			return err
		}

		if account.UserID != userID {
			return domainerrors.ErrForbidden
		}

		if err := validateAccountAmounts(account.Balance, account.ReservedAmount); err != nil {
			return err
		}

		if account.Balance-account.ReservedAmount < amount {
			return domainerrors.ErrInsufficientFunds
		}

		balanceBefore := account.Balance
		balanceAfter := balanceBefore - amount

		if err := service.accounts.UpdateBalance(ctx, tx, account.ID, balanceAfter); err != nil {
			return err
		}

		entry, err = service.operations.Create(ctx, tx, entity.LedgerEntry{
			AccountID:     account.ID,
			Type:          entity.LedgerEntryTypeWithdrawal,
			Amount:        amount,
			Currency:      account.Currency,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
		})
		return err
	})
	if err != nil {
		return entity.LedgerEntry{}, err
	}

	return entry, nil
}

func (service *accountService) ListOperations(
	ctx context.Context,
	userID int64,
	accountID int64,
	limit int,
	offset int,
) ([]entity.LedgerEntry, error) {
	if _, err := service.GetByID(ctx, userID, accountID); err != nil {
		return nil, err
	}

	return service.operations.ListByAccount(ctx, accountID, limit, offset)
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func isValidCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}

	for _, symbol := range currency {
		if symbol < 'A' || symbol > 'Z' {
			return false
		}
	}

	return true
}

func validateAccountAmounts(balance int64, reservedAmount int64) error {
	if balance < 0 {
		return fmt.Errorf("account balance cannot be negative: %w", domainerrors.ErrNegativeBalance)
	}

	if reservedAmount < 0 {
		return fmt.Errorf("account reserved amount cannot be negative: %w", domainerrors.ErrNegativeReservedAmount)
	}

	if reservedAmount > balance {
		return fmt.Errorf("account reserved amount cannot exceed balance: %w", domainerrors.ErrReservedAmountExceedsBalance)
	}

	return nil
}
