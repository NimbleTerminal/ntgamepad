// Copyright (c) 2026 Neomantra Corp.

//go:build darwin && !js

package ntgamepad

import (
	"runtime"
	"syscall"
	"testing"
)

// A poll switches on background monitoring, without which a program that is
// never the frontmost app reads every controller at rest. Older systems
// without the property are skipped.
func TestDarwinPollEnablesBackgroundMonitoring(t *testing.T) {
	r := Open()
	defer r.Close()
	_ = r.Poll()

	b := &bridge
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ok {
		t.Skip("GameController.framework did not load")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := b.pushPool()
	defer b.popPool(pool)
	if !b.responds(b.clsGCController, b.selBackground) {
		t.Skip("GCController has no shouldMonitorBackgroundEvents (before macOS 11.3)")
	}
	if !b.sendBool(b.clsGCController, b.selBackground) {
		t.Fatal("shouldMonitorBackgroundEvents is still off after a poll")
	}
}

// Polling at frame rate does not grow the process: the objects each poll
// autoreleases are freed with its pool. Measured with no controller attached,
// 400,000 polls grow the resident set by about 1 MB with the pool and 22 MB
// without it. The first polls are left out: they warm up the Go heap.
func TestDarwinPollingDoesNotLeak(t *testing.T) {
	r := Open()
	defer r.Close()
	var pads []State
	for range 200000 {
		pads = r.PollAll(pads)
	}
	before := maxRSS(t)
	const polls = 400000
	for range polls {
		pads = r.PollAll(pads)
	}
	grew := maxRSS(t) - before
	t.Logf("resident set grew %d KB over %d polls", grew/1024, polls)
	if grew > 6<<20 {
		t.Fatalf("resident set grew %d KB over %d polls, want under 6 MB", grew/1024, polls)
	}
}

// maxRSS is the process's peak resident set size in bytes.
func maxRSS(t *testing.T) int64 {
	t.Helper()
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		t.Fatalf("getrusage: %v", err)
	}
	return ru.Maxrss // bytes on darwin
}
