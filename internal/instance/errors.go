package instance

import "errors"

var ErrAlreadyRunning = errors.New("another CLIProxyAPI Desktop instance is using this data directory")
