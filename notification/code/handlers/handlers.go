package handlers

import (
	_ "work/notification/code/handlers/sample_aggregate_only"
	_ "work/notification/code/handlers/sample_both"
	_ "work/notification/code/handlers/sample_realtime_only"
)

// Register imports all built-in handlers so their init registrations run.
func Register() {}
