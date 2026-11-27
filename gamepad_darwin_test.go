// Copyright (c) 2026 Neomantra Corp.

//go:build darwin && !js

package ntgamepad

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
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

// mainOnMainThread is whether the main goroutine was on the main thread when
// the tests started; TestMain runs on the main goroutine.
var mainOnMainThread, frameworkLoaded bool

func TestMain(m *testing.M) {
	if os.Getenv("NTGAMEPAD_TEST_RUN_LOOP_CALLBACK") == "1" {
		if err := pollFromMainQueue(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	bridge.mu.Lock()
	if frameworkLoaded = bridge.load(); frameworkLoaded {
		mainOnMainThread = bridge.onMainThread()
	}
	bridge.mu.Unlock()
	os.Exit(m.Run())
}

// A callback dispatched by Poll can read Notes and poll another reader.
// The child owns its main thread, and a regression cannot hang the test suite.
func TestDarwinRunLoopCallbackCanPoll(t *testing.T) {
	if !frameworkLoaded {
		t.Skip("GameController.framework did not load")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), "NTGAMEPAD_TEST_RUN_LOOP_CALLBACK=1", "NTGAMEPAD_HID=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("main-queue callback: %v\n%s", err, out)
	}
}

func pollFromMainQueue() error {
	reader := Open()
	defer reader.Close()
	lib, err := ffi.LoadLibrary("/usr/lib/libSystem.B.dylib")
	if err != nil {
		return err
	}
	queue, err := ffi.GetSymbol(lib, "_dispatch_main_q") // dispatch_get_main_queue()
	if err != nil {
		return err
	}
	dispatch, err := ffi.GetSymbol(lib, "dispatch_async_f")
	if err != nil {
		return err
	}
	var cif types.CallInterface
	ptr := types.PointerTypeDescriptor
	if err := ffi.PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr, ptr, ptr}); err != nil {
		return err
	}
	called := false
	callback := ffi.NewCallback(func(_ uintptr) {
		_ = Notes()
		nested := Open()
		_ = nested.Poll()
		_ = nested.Close()
		called = true
	})
	var arg uintptr
	if _, err := ffi.CallFunction(&cif, dispatch, nil, []unsafe.Pointer{unsafe.Pointer(&queue), unsafe.Pointer(&arg), unsafe.Pointer(&callback)}); err != nil {
		return err
	}
	for !called {
		reader.Poll()
	}
	return nil
}

// Importing the package keeps the main goroutine on the main thread, which
// is what lets a poll from it run the main run loop; a test's own goroutine
// is not on it.
func TestDarwinMainGoroutineIsOnTheMainThread(t *testing.T) {
	if !frameworkLoaded {
		t.Skip("GameController.framework did not load")
	}
	if !mainOnMainThread {
		t.Fatal("the main goroutine is not on the main thread")
	}
	bridge.mu.Lock()
	here := bridge.onMainThread()
	bridge.mu.Unlock()
	if here {
		t.Fatal("a test goroutine reports being on the main thread")
	}
}

// A program that polls only from another goroutine is told so.
func TestDarwinNotesSayWhenPollsAreOffTheMainGoroutine(t *testing.T) {
	if !frameworkLoaded {
		t.Skip("GameController.framework did not load")
	}
	r := Open()
	defer r.Close()
	_ = r.Poll()
	found := false
	for _, n := range Notes() {
		found = found || n.Code == NotePollOffMainGoroutine
	}
	if !found {
		t.Fatalf("notes = %v, want the off-main-goroutine note", Notes())
	}
}
