package core

import (
	"errors"
	"fmt"
)

var ErrInvalidConfType = func(s string) error {
	return fmt.Errorf("invalid configuration type (%s)", s)
}

var ErrNoTargetAvailable = errors.New("empty target list in configuration file")
var ErrArgNoTargetSpecified = errors.New("empty target argument(s)")
