package service

import "errors"

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrForbidden         = errors.New("role is not permitted for this operation")
	ErrTwoPersonRequired = errors.New("submitter and approver must be different users")
	ErrImmutableState    = errors.New("record can no longer be edited in its current state")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
)
