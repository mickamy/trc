package main

import (
	"context"
	"fmt"
)

func handleWatch(_ context.Context, _ globalFlags, _ []string) error {
	return fmt.Errorf("not implemented: watch")
}

func handleTree(_ context.Context, _ globalFlags, _ []string) error {
	return fmt.Errorf("not implemented: tree")
}

func handleMap(_ context.Context, _ globalFlags, _ []string) error {
	return fmt.Errorf("not implemented: map")
}
