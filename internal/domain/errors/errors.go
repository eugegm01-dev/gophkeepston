package errors

import "errors"

var (
	ErrNotFound      = errors.New("entry not found")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrConflict      = errors.New("version conflict")
	ErrCorruptedData = errors.New("corrupted data")
)
