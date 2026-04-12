package errors

type DomainError string

func (err DomainError) Error() string {
	return string(err)
}

const (
	ErrAccountNotFound      DomainError = "account not found"
	ErrAccountAlreadyExists DomainError = "account already exists"
	ErrForbidden            DomainError = "forbidden"
	ErrInvalidCurrency      DomainError = "invalid currency"
)
