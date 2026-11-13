// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc

/*
#include <stdint.h>
#include "rmdoc/rmdoc.h"
*/
import "C"

import (
	"runtime"
	"runtime/cgo"
	"sync/atomic"
)

// visit is what one ForEach or Unpack call hands its C callbacks, through a
// cgo.Handle. Caller code runs on a helper goroutine, one callback at a time.
// Fields it writes are read only after the wait, so they need no lock.
type visit struct {
	selectFn func(Entry) bool
	body     func(Entry, []byte) error

	// err is what the first failed callback failed with.
	err error
	// panicked and panicValue keep a callback's panic, recovered before it
	// can unwind through C and raised again once C has returned.
	panicked   bool
	panicValue any
	// exited keeps Goexit until the same cleanup has run.
	exited bool
}

// liveHandles counts calls whose callback state has not yet been released.
var liveHandles atomic.Int64

// fail records the first failure of a callback.
func (v *visit) fail(err error) {
	if v.err == nil && !v.panicked && !v.exited {
		v.err = err
	}
}

// invoke keeps a callback's panic or exit until C has stopped and released
// its memory. Only the caller's code runs on the helper goroutine.
func (v *visit) invoke(fn func()) bool {
	returned, p := runCallback(fn)
	v.panicked = p != nil
	v.panicValue = p
	v.exited = !returned && p == nil
	return returned
}

// raise panics again with a panic a callback recovered from, now that no C
// frame is left between it and the caller.
func (v *visit) raise() {
	if v.panicked {
		panic(v.panicValue)
	}
	if v.exited {
		runtime.Goexit()
	}
}

// run calls C with a handle to v, deleting the handle afterwards and raising
// any panic or exit of a callback.
func (v *visit) run(call func(handle C.uintptr_t) C.rmd_status) C.rmd_status {
	liveHandles.Add(1)
	h := cgo.NewHandle(v)
	status := call(C.uintptr_t(h))
	h.Delete()
	liveHandles.Add(-1)
	v.raise()
	return status
}

// callbackError answers the error a callback failure becomes: an *Error of
// StatusCallback that unwraps to what the callback failed with.
func (v *visit) callbackError(status C.rmd_status, cerr *C.rmd_error) error {
	if status == C.RMD_OK {
		return nil
	}
	e := cError(status, cerr)
	if e.Status == StatusCallback {
		e.Err = v.err
	}
	return e
}

// rmdgoSelect answers 1 to include an entry, 0 to skip it and -1 to stop.
//
//export rmdgoSelect
func rmdgoSelect(handle C.uintptr_t, entry *C.rmd_entry) (answer C.int) {
	v := cgo.Handle(handle).Value().(*visit)
	e, err := entryFromC(entry)
	if err != nil {
		v.fail(err)
		return -1
	}
	var selected bool
	if !v.invoke(func() { selected = v.selectFn(e) }) {
		return -1
	}
	if selected {
		return 1
	}
	return 0
}

// rmdgoBody answers 0 to go on and -1 to stop.
//
//export rmdgoBody
func rmdgoBody(handle C.uintptr_t, entry *C.rmd_entry, data *C.uint8_t, size C.size_t) (answer C.int) {
	v := cgo.Handle(handle).Value().(*visit)
	e, err := entryFromC(entry)
	if err != nil {
		v.fail(err)
		return -1
	}
	payload, err := borrowedBytes(data, size)
	if err != nil {
		v.fail(err)
		return -1
	}
	if !v.invoke(func() { err = v.body(e, payload) }) {
		return -1
	}
	if err != nil {
		v.fail(err)
		return -1
	}
	return 0
}

// runCallback waits for fn on a separate goroutine so neither a panic nor
// Goexit can unwind the C frames of the goroutine waiting for it.
func runCallback(fn func()) (returned bool, panicValue any) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { panicValue = recover() }()
		fn()
		returned = true
	}()
	<-done
	return returned, panicValue
}
