package main

import (
	"context"
	"errors"
)

func handleWatch(_ context.Context, _ globalFlags, _ []string) error {
	return errors.New("not implemented: watch")
}

func handleTree(_ context.Context, _ globalFlags, _ []string) error {
	return errors.New("not implemented: tree")
}

func handleMap(_ context.Context, _ globalFlags, _ []string) error {
	return errors.New("not implemented: map")
}
