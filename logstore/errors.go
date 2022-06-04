package logstore

import (
	"errors"
	"fmt"
)

var (
	// helper variables to implement the packages .IsXYError() functions
	notFoundErr   *NotFoundError
	badRequestErr *BadRequestError
)

// NotFoundError indicates that no value for a key is stored in the db
type NotFoundError struct {
	key string
}

func NewNotFoundError(key string) error {
	return &NotFoundError{key}
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no value for key: %v", e.key)
}

func IsNotFoundError(err error) bool {
	return errors.As(err, &notFoundErr)
}

// BadRequestError indicates that the value provided was not writable to the database
type BadRequestError struct {
	reason string
}

func NewBadRequestError(reason string) error {
	return &BadRequestError{reason}
}

func (b *BadRequestError) Error() string {
	return fmt.Sprintf("invalid write request (reason: %v)", b.reason)
}

func IsBadRequestError(err error) bool {
	return errors.As(err, &badRequestErr)
}
