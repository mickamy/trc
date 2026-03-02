package main

import "time"

type globalFlags struct {
	network  string
	filter   string
	duration time.Duration
}
