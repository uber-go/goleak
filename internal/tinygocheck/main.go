// Copyright (c) 2026 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:build tinygo

// Command tinygocheck is a smoke test that verifies goleak builds and runs
// under TinyGo without panicking.
//
// TinyGo's runtime.Stack produces no output that goleak can parse, which used
// to make goleak panic with an index-out-of-range error. goleak now detects
// that and skips its checks instead; this program exercises that path.
//
// It is built only by TinyGo, which defines the "tinygo" build tag; standard
// Go tooling (build, test, vet, lint, coverage) ignores this file.
package main

import (
	"fmt"
	"os"

	"go.uber.org/goleak"
)

// recordingT implements goleak.TestingT and the optional Logf method, so we
// can observe whether goleak fails the check or merely skips it.
type recordingT struct {
	failed bool
}

func (t *recordingT) Error(args ...any) {
	t.failed = true
	fmt.Println("goleak reported:", fmt.Sprint(args...))
}

func (t *recordingT) Logf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

type noopM struct{}

func (noopM) Run() int { return 0 }

func fail(msg string) {
	fmt.Println("FAIL:", msg)
	os.Exit(1)
}

func main() {
	// None of the calls below must panic under TinyGo.

	// Find reports that stacks are unavailable rather than crashing.
	if err := goleak.Find(); err != nil {
		fmt.Println("Find:", err)
	}

	// VerifyNone must skip (not fail) when stacks can't be parsed.
	rt := &recordingT{}
	goleak.VerifyNone(rt)
	if rt.failed {
		fail("VerifyNone reported a leak under TinyGo")
	}

	// IgnoreCurrent must also degrade gracefully.
	goleak.VerifyNone(rt, goleak.IgnoreCurrent())
	if rt.failed {
		fail("VerifyNone(IgnoreCurrent) reported a leak under TinyGo")
	}

	// VerifyTestMain must not panic. Cleanup overrides the default os.Exit so
	// this program stays in control of its own exit.
	goleak.VerifyTestMain(noopM{}, goleak.Cleanup(func(int) {}))

	fmt.Println("PASS: goleak ran under TinyGo without panicking")
}
