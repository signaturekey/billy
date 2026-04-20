package errors

type DomainError string

func (err DomainError) Error() string {
	return string(err)
}

const (
	ErrAccountNotFound              DomainError = "account not found"
	ErrAccountAlreadyExists         DomainError = "account already exists"
	ErrForbidden                    DomainError = "forbidden"
	ErrInvalidCurrency              DomainError = "invalid currency"
	ErrInvalidAmount                DomainError = "invalid amount"
	ErrInsufficientFunds            DomainError = "insufficient funds"
	ErrSameAccountTransfer          DomainError = "same account transfer"
	ErrCurrencyMismatch             DomainError = "currency mismatch"
	ErrAccountBlocked               DomainError = "account blocked"
	ErrHoldNotFound                 DomainError = "hold not found"
	ErrHoldExpired                  DomainError = "hold expired"
	ErrHoldAlreadyConfirmed         DomainError = "hold already confirmed"
	ErrHoldAlreadyCancelled         DomainError = "hold already cancelled"
	ErrInvalidHoldStateTransition   DomainError = "invalid hold state transition"
	ErrNegativeBalance              DomainError = "negative balance"
	ErrNegativeReservedAmount       DomainError = "negative reserved amount"
	ErrReservedAmountExceedsBalance DomainError = "reserved amount exceeds balance"
)
