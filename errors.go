package split_openfeature_provider_go

import "errors"

// ErrNilSplitClient is returned when NewProvider is called with a nil Split client.
var ErrNilSplitClient = errors.New("Split client cannot be nil")

// errNilSplitClient is an alias for ErrNilSplitClient for internal use.
var errNilSplitClient = ErrNilSplitClient
