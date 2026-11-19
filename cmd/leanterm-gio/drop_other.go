//go:build !windows

package main

import "gioui.org/app"

func hwndOf(app.ViewEvent) uintptr { return 0 }
