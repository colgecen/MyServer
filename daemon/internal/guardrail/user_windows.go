//go:build windows

package guardrail

import "os"

func osUser() string { return os.Getenv("USERNAME") }
