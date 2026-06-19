package entity

import "errors"

// ErrNotFound is a sentinel error indicating a requested resource does not
// exist. Producers wrap it (e.g. fmt.Errorf("user %w", entity.ErrNotFound))
// so handlers can map it to HTTP 404 via errors.Is, without comparing
// message strings.
var ErrNotFound = errors.New("not found")
