package catalog

import "errors"

// ErrNotFound means the catalog entry does not exist.
var ErrNotFound = errors.New("catalog entry not found")
