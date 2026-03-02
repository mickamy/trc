package main

import "time"

type globalFlags struct {
	network  string
	filter   string
	duration time.Duration
	output   string // -o: save raw NDJSON during watch
	input    string // -i: read NDJSON file for tree/map
}
