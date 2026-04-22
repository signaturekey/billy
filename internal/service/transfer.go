package service

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

type TransferRepository interface {
	Create(ctx context.Context, tx pgx.Tx, transfer entity.Transfer) (entity.Transfer, error)
}

type transferService struct {
	txManager  TxManager
	accounts   AccountRepository
	transfers  TransferRepository
	operations LedgerRepository
}

func NewTransferService(
	txManager TxManager,
	accounts AccountRepository,
	transfers TransferRepository,
	operations LedgerRepository,
) *transferService {
	return &transferService{
		txManager:  txManager,
		accounts:   accounts,
		transfers:  transfers,
		operations: operations,
	}
}

func (service *transferService) Create(
	ctx context.Context,
	userID int64,
	fromAccountID int64,
	toAccountID int64,
	amount int64,
) (entity.Transfer, error) {
	var transfer entity.Transfer
	err := service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		transfer, err = service.CreateInTx(ctx, tx, userID, fromAccountID, toAccountID, amount)
		return err
	})
	if err != nil {
		return entity.Transfer{}, err
	}

	return transfer, nil
}

func (service *transferService) CreateInTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	fromAccountID int64,
	toAccountID int64,
	amount int64,
) (entity.Transfer, error) {
	if amount <= 0 {
		return entity.Transfer{}, domainerrors.ErrInvalidAmount
	}

	if fromAccountID == toAccountID {
		return entity.Transfer{}, domainerrors.ErrSameAccountTransfer
	}

	firstID, secondID := orderedAccountIDs(fromAccountID, toAccountID)

	first, err := service.accounts.GetForUpdate(ctx, tx, firstID)
	if err != nil {
		return entity.Transfer{}, err
	}

	second, err := service.accounts.GetForUpdate(ctx, tx, secondID)
	if err != nil {
		return entity.Transfer{}, err
	}

	from, to := resolveTransferAccounts(first, second, fromAccountID)

	if from.UserID != userID {
		return entity.Transfer{}, domainerrors.ErrForbidden
	}

	if from.Status != entity.AccountStatusActive || to.Status != entity.AccountStatusActive {
		return entity.Transfer{}, domainerrors.ErrAccountBlocked
	}

	if from.Currency != to.Currency {
		return entity.Transfer{}, domainerrors.ErrCurrencyMismatch
	}

	if err := validateAccountAmounts(from.Balance, from.ReservedAmount); err != nil {
		return entity.Transfer{}, err
	}

	if err := validateAccountAmounts(to.Balance, to.ReservedAmount); err != nil {
		return entity.Transfer{}, err
	}

	if from.Balance-from.ReservedAmount < amount {
		return entity.Transfer{}, domainerrors.ErrInsufficientFunds
	}

	fromBalanceBefore := from.Balance
	fromBalanceAfter := fromBalanceBefore - amount
	toBalanceBefore := to.Balance
	toBalanceAfter := toBalanceBefore + amount

	if err := service.accounts.UpdateBalance(ctx, tx, from.ID, fromBalanceAfter); err != nil {
		return entity.Transfer{}, err
	}

	if err := service.accounts.UpdateBalance(ctx, tx, to.ID, toBalanceAfter); err != nil {
		return entity.Transfer{}, err
	}

	transfer, err := service.transfers.Create(ctx, tx, entity.Transfer{
		FromAccountID: from.ID,
		ToAccountID:   to.ID,
		Amount:        amount,
		Status:        entity.TransferStatusCompleted,
	})
	if err != nil {
		return entity.Transfer{}, err
	}

	if _, err := service.operations.Create(ctx, tx, entity.LedgerEntry{
		AccountID:     from.ID,
		Type:          entity.LedgerEntryTypeTransferOut,
		Amount:        amount,
		Currency:      from.Currency,
		BalanceBefore: fromBalanceBefore,
		BalanceAfter:  fromBalanceAfter,
		ReferenceType: "transfer",
		ReferenceID:   transfer.ID,
	}); err != nil {
		return entity.Transfer{}, err
	}

	if _, err := service.operations.Create(ctx, tx, entity.LedgerEntry{
		AccountID:     to.ID,
		Type:          entity.LedgerEntryTypeTransferIn,
		Amount:        amount,
		Currency:      to.Currency,
		BalanceBefore: toBalanceBefore,
		BalanceAfter:  toBalanceAfter,
		ReferenceType: "transfer",
		ReferenceID:   transfer.ID,
	}); err != nil {
		return entity.Transfer{}, err
	}

	return transfer, nil
}

func orderedAccountIDs(first int64, second int64) (int64, int64) {
	if first < second {
		return first, second
	}

	return second, first
}

func resolveTransferAccounts(first entity.Account, second entity.Account, fromAccountID int64) (entity.Account, entity.Account) {
	if first.ID == fromAccountID {
		return first, second
	}

	return second, first
}
