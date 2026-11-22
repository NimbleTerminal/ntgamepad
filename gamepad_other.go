// Copyright (c) 2026 Neomantra Corp.

//go:build !js && !darwin && !linux

package ntgamepad

// noopReader is the Reader on a platform without support (Windows among
// them): it reports no controllers.
type noopReader struct{}

func openPlatform() Reader { return noopReader{} }

func (noopReader) Poll() State                 { return State{} }
func (noopReader) PollAll(dst []State) []State { return dst[:0] }
func (noopReader) Close() error                { return nil }
