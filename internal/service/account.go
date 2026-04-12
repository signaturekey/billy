package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/signaturekey/billy/internal/domain/entity"
	"github.com/signaturekey/billy/internal/domain/errors"
)

type AccountRepository interface {
	Create(ctx context.Context, account entity.Account) (entity.Account, error)
	GetByID(ctx context.Context, id int64) (entity.Account, error)
}

type accountService struct {
	repository AccountRepository
}

func NewAccountService(repository AccountRepository) *accountService {
	return &accountService{repository: repository}
}

func (service *accountService) Create(ctx context.Context, userID int64, currency string) (entity.Account, error) {
	normalizedCurrency := normalizeCurrency(currency)
	if !isValidCurrency(normalizedCurrency) {
		return entity.Account{}, errors.ErrInvalidCurrency
	}

	account := entity.Account{
		UserID:         userID,
		Currency:       normalizedCurrency,
		Balance:        0,
		ReservedAmount: 0,
		Status:         entity.AccountStatusActive,
	}

	createdAccount, err := service.repository.Create(ctx, account)
	if err != nil {
		return entity.Account{}, err
	}

	if err := validateAccountAmounts(createdAccount.Balance, createdAccount.ReservedAmount); err != nil {
		return entity.Account{}, err
	}

	return createdAccount, nil
}

func (service *accountService) GetByID(ctx context.Context, userID int64, accountID int64) (entity.Account, error) {
	account, err := service.repository.GetByID(ctx, accountID)
	if err != nil {
		return entity.Account{}, err
	}

	if account.UserID != userID {
		return entity.Account{}, errors.ErrForbidden
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
		return fmt.Errorf("account balance cannot be negative")
	}

	if reservedAmount < 0 {
		return fmt.Errorf("account reserved amount cannot be negative")
	}

	if reservedAmount > balance {
		return fmt.Errorf("account reserved amount cannot exceed balance")
	}

	return nil
}
