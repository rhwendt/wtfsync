//go:build !windows

package autostart

import "errors"

func (Run) Enabled() (bool, error) { return false, nil }

func (Run) Set(bool) error { return errors.New("start at login is only supported on Windows") }
