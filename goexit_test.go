// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc_test

import (
	"bytes"
	"fmt"
	"github.com/altessa-s/rmdoc-go"
	"math/rand/v2"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
)

func TestCallbackGoexitCleansUp(t *testing.T) {
	r := open(t, packed(t), nil)
	before := rmdoc.LiveHandles()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = r.ForEach(nil, func(rmdoc.Entry, []byte) error { runtime.Goexit(); return nil })
		t.Error("ForEach returned after Goexit")
	}()
	<-done
	if got := rmdoc.LiveHandles(); got != before {
		t.Fatalf("live handles = %d, want %d", got, before)
	}
}

func TestCallbacksStopOnSecondExit(t *testing.T) {
	r := open(t, packed(t), nil)
	for _, which := range []string{"select", "body", "unpack"} {
		t.Run(which, func(t *testing.T) {
			calls := 0
			before := rmdoc.LiveHandles()
			done := make(chan struct{})
			next := func() {
				calls++
				if calls == 2 {
					runtime.Goexit()
				}
			}
			go func() {
				defer close(done)
				switch which {
				case "select":
					_ = r.ForEach(func(rmdoc.Entry) bool { next(); return true }, func(rmdoc.Entry, []byte) error { t.Error("body called after select exited"); return nil })
				case "body":
					_ = r.ForEach(nil, func(rmdoc.Entry, []byte) error { next(); return nil })
				case "unpack":
					_, _ = r.Unpack(func(rmdoc.Entry) bool { next(); return true })
				}
				t.Error("call returned after exit")
			}()
			<-done
			if calls != 2 || rmdoc.LiveHandles() != before {
				t.Fatalf("calls = %d, live handles = %d, want 2 and %d", calls, rmdoc.LiveHandles(), before)
			}
		})
	}
}

func TestCallbackAbnormalConcurrency(t *testing.T) {
	r := open(t, packed(t), nil)
	before := rmdoc.LiveHandles()
	random := rand.New(rand.NewPCG(1, 2))
	var wg sync.WaitGroup
	for range 256 {
		panics := random.IntN(2) == 0
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); (panics && p != "stop") || (!panics && p != nil) {
					t.Errorf("panic = %v, panics = %v", p, panics)
				}
			}()
			_ = r.ForEach(nil, func(rmdoc.Entry, []byte) error {
				if panics {
					panic("stop")
				}
				runtime.Goexit()
				return nil
			})
			t.Error("abnormal callback returned")
		}()
	}
	wg.Wait()
	if got := rmdoc.LiveHandles(); got != before {
		t.Fatalf("live handles = %d, want %d", got, before)
	}
}

func TestRepeatedCallbackExit(t *testing.T) {
	r := open(t, packed(t), nil)
	before := rmdoc.LiveHandles()
	for range 2000 {
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = r.Unpack(func(rmdoc.Entry) bool { runtime.Goexit(); return true })
		}()
		<-done
		if got := rmdoc.LiveHandles(); got != before {
			t.Fatalf("live handles = %d, want %d", got, before)
		}
	}
}

// A failed subtest also fails its parent, so exercise Fatal in a child test
// process and let the outer test check both its failure and its cleanup.
func TestCallbackFatal(t *testing.T) {
	if os.Getenv("RMDOC_CALLBACK_FATAL") == "1" {
		r := open(t, packed(t), nil)
		before := rmdoc.LiveHandles()
		deferred := false
		ok := t.Run("callback", func(t *testing.T) {
			defer func() { deferred = true }()
			_ = r.ForEach(nil, func(rmdoc.Entry, []byte) error { t.Fatal("callback fatal"); return nil })
		})
		if !ok && deferred && rmdoc.LiveHandles() == before {
			fmt.Println("fatal failed and cleaned up")
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCallbackFatal$")
	cmd.Env = append(os.Environ(), "RMDOC_CALLBACK_FATAL=1")
	out, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("fatal failed and cleaned up")) {
		t.Fatalf("child = %v: %s", err, out)
	}
}
