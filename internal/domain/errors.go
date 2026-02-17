package domain

import "errors"

// Sentinel errors for the domain layer.
var (
	ErrNotFound     = errors.New("domain: not found")
	ErrConflict     = errors.New("domain: conflict")
	ErrUnauthorized = errors.New("domain: unauthorized")
	ErrForbidden    = errors.New("domain: forbidden")
)
