//go:build !windows

package main

import "time"

func windowTitleAt(x, y int) string { return "" }

func hideLive(wait time.Duration) func() { return func() {} }
