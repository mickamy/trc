package main

type globalFlags struct {
	network string
	filter  string
	input   string // -i: read NDJSON file into TUI
}
