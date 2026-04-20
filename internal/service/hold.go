package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

type HoldRepository interface {
	Create(ctx context.Context, tx pgx.Tx, hold entity.Hold) (entity.Hold, error)
	GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id int64) (entity.Hold, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, id int64, status entity.HoldStatus) (entity.Hold, error)
}

type holdService struct {
	txManager  TxManager
	accounts   AccountRepository
	holds      HoldRepository
	operations LedgerRepository
	ttl        time.Duration
}

func NewHoldService(
	txManager TxManager,
	accounts AccountRepository,
	holds HoldRepository,
	operations LedgerRepository,
	ttl time.Duration,
) *holdService {
	return &holdService{
		txManager:  txManager,
		accounts:   accounts,
		holds:      holds,
		operations: operations,
		ttl:        ttl,
	}
}

func (service *holdService) Create(
	ctx context.Context,
	userID int64,
	accountID int64,
	amount int64,
) (entity.Hold, error) {
	if amount <= 0 {
		return entity.Hold{}, domainerrors.ErrInvalidAmount
	}

	var hold entity.Hold
	err := service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		account, err := service.accounts.GetForUpdate(ctx, tx, accountID)
		if err != nil {
			return err
		}

		if err := validateHoldAccount(account, userID); err != nil {
			return err
		}

		if account.Balance-account.ReservedAmount < amount {
			return domainerrors.ErrInsufficientFunds
		}

		if err := service.accounts.UpdateAmounts(
			ctx,
			tx,
			account.ID,
			account.Balance,
			account.ReservedAmount+amount,
		); err != nil {
			return err
		}

		hold, err = service.holds.Create(ctx, tx, entity.Hold{
			AccountID: account.ID,
			Amount:    amount,
			Status:    entity.HoldStatusPending,
			ExpiresAt: time.Now().Add(service.ttl),
		})
		return err
	})
	if err != nil {
		return entity.Hold{}, err
	}

	return hold, nil
}

func (service *holdService) Confirm(ctx context.Context, userID int64, holdID int64) (entity.Hold, error) {
	var updated entity.Hold
	err := service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		hold, account, err := service.lockHoldAndAccount(ctx, tx, userID, holdID)
		if err != nil {
			return err
		}

		if err := validatePendingHold(hold); err != nil {
			return err
		}

		if !time.Now().Before(hold.ExpiresAt) {
			return domainerrors.ErrHoldExpired
		}

		if account.ReservedAmount < hold.Amount || account.Balance < hold.Amount {
			return domainerrors.ErrInvalidHoldStateTransition
		}

		balanceAfter := account.Balance - hold.Amount
		reservedAfter := account.ReservedAmount - hold.Amount
		if err := service.accounts.UpdateAmounts(ctx, tx, account.ID, balanceAfter, reservedAfter); err != nil {
			return err
		}

		updated, err = service.holds.UpdateStatus(ctx, tx, hold.ID, entity.HoldStatusConfirmed)
		if err != nil {
			return err
		}

		_, err = service.operations.Create(ctx, tx, entity.LedgerEntry{
			AccountID:     account.ID,
			Type:          entity.LedgerEntryTypeHoldConfirm,
			Amount:        hold.Amount,
			Currency:      account.Currency,
			BalanceBefore: account.Balance,
			BalanceAfter:  balanceAfter,
			ReferenceType: "hold",
			ReferenceID:   hold.ID,
		})
		return err
	})
	if err != nil {
		return entity.Hold{}, err
	}

	return updated, nil
}

func (service *holdService) Cancel(ctx context.Context, userID int64, holdID int64) (entity.Hold, error) {
	var updated entity.Hold
	err := service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		hold, account, err := service.lockHoldAndAccount(ctx, tx, userID, holdID)
		if err != nil {
			return err
		}

		if err := validatePendingHold(hold); err != nil {
			return err
		}

		if account.ReservedAmount < hold.Amount {
			return domainerrors.ErrInvalidHoldStateTransition
		}

		if err := service.accounts.UpdateAmounts(
			ctx,
			tx,
			account.ID,
			account.Balance,
			account.ReservedAmount-hold.Amount,
		); err != nil {
			return err
		}

		updated, err = service.holds.UpdateStatus(ctx, tx, hold.ID, entity.HoldStatusCancelled)
		return err
	})
	if err != nil {
		return entity.Hold{}, err
	}

	return updated, nil
}

func (service *holdService) lockHoldAndAccount(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	holdID int64,
) (entity.Hold, entity.Account, error) {
	hold, err := service.holds.GetByIDForUpdate(ctx, tx, holdID)
	if err != nil {
		return entity.Hold{}, entity.Account{}, err
	}

	account, err := service.accounts.GetForUpdate(ctx, tx, hold.AccountID)
	if err != nil {
		return entity.Hold{}, entity.Account{}, err
	}

	if err := validateHoldAccount(account, userID); err != nil {
		return entity.Hold{}, entity.Account{}, err
	}

	return hold, account, nil
}

func validateHoldAccount(account entity.Account, userID int64) error {
	if account.UserID != userID {
		return domainerrors.ErrForbidden
	}

	if account.Status != entity.AccountStatusActive {
		return domainerrors.ErrAccountBlocked
	}

	return validateAccountAmounts(account.Balance, account.ReservedAmount)
}

func validatePendingHold(hold entity.Hold) error {
	switch hold.Status {
	case entity.HoldStatusPending:
		return nil
	case entity.HoldStatusConfirmed:
		return domainerrors.ErrHoldAlreadyConfirmed
	case entity.HoldStatusCancelled:
		return domainerrors.ErrHoldAlreadyCancelled
	case entity.HoldStatusExpired:
		return domainerrors.ErrHoldExpired
	default:
		return domainerrors.ErrInvalidHoldStateTransition
	}
}
