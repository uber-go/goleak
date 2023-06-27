// Copyright (c) 2017 Uber Technologies, Inc.
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

package goleak

import (
	"errors"
	"strings"
	"time"

	"go.uber.org/goleak/internal/stack"
)

// Option lets users specify custom verifications.
type Option interface {
	apply(*opts)
}

const (
	// We retry up to default 20 times if we can't find the goroutine that
	// we are looking for.
	_defaultRetryAttempts = 20
	// In between each retry attempt, sleep for up to default 100 microseconds
	// to let any running goroutine completes.
	_defaultSleepInterval = 100 * time.Microsecond
)

type opts struct {
	filters    []func(stack.Stack) bool
	maxRetries int
	maxSleep   time.Duration
	cleanup    func(int)
}

// implement apply so that opts struct itself can be used as
// an Option.
func (o *opts) apply(opts *opts) {
	opts.filters = o.filters
	opts.maxRetries = o.maxRetries
	opts.maxSleep = o.maxSleep
	opts.cleanup = o.cleanup
}

// validate the options.
func (o *opts) validate() error {
	if o.maxRetries < 0 {
		return errors.New("maxRetryAttempts should be greater than 0")
	}
	if o.maxSleep <= 0 {
		return errors.New("maxSleepInterval should be greater than 0s")
	}
	return nil
}

// optionFunc lets us easily write options without a custom type.
type optionFunc func(*opts)

func (f optionFunc) apply(opts *opts) { f(opts) }

// IgnoreTopFunction ignores any goroutines where the specified function
// is at the top of the stack. The function name should be fully qualified,
// e.g., go.uber.org/goleak.IgnoreTopFunction
func IgnoreTopFunction(f string) Option {
	return addFilter(func(s stack.Stack) bool {
		return s.FirstFunction() == f
	})
}

// Cleanup sets up a cleanup function that will be executed at the
// end of the leak check.
// When passed to [VerifyTestMain], the exit code passed to cleanupFunc
// will be set to the exit code of TestMain.
// When passed to [VerifyNone], the exit code will be set to 0.
// This cannot be passed to [Find].
func Cleanup(cleanupFunc func(exitCode int)) Option {
	return optionFunc(func(opts *opts) {
		opts.cleanup = cleanupFunc
	})
}

// IgnoreCurrent records all current goroutines when the option is created, and ignores
// them in any future Find/Verify calls.
func IgnoreCurrent() Option {
	excludeIDSet := map[int]bool{}
	for _, s := range stack.All() {
		excludeIDSet[s.ID()] = true
	}
	return addFilter(func(s stack.Stack) bool {
		return excludeIDSet[s.ID()]
	})
}

// MaxSleepInterval sets the maximum sleep time in-between each retry attempt.
// The sleep duration grows in an exponential backoff, to a maximum of the value specified here.
// If not configured, default to 100 microseconds.
func MaxSleepInterval(d time.Duration) Option {
	return optionFunc(func(opts *opts) {
		opts.maxSleep = d
	})
}

// MaxRetryAttempts sets the retry upper limit.
// When finding extra goroutines, we'll retry until all goroutines complete
// or end up with the maximum retry attempts.
// If not configured, default to 20 times.
func MaxRetryAttempts(num int) Option {
	return optionFunc(func(opts *opts) {
		opts.maxRetries = num
	})
}

func addFilter(f func(stack.Stack) bool) Option {
	return optionFunc(func(opts *opts) {
		opts.filters = append(opts.filters, f)
	})
}

func buildOpts(options ...Option) *opts {
	opts := &opts{
		maxRetries: _defaultRetryAttempts,
		maxSleep:   _defaultSleepInterval,
	}
	opts.filters = append(opts.filters,
		isTestStack,
		isSyscallStack,
		isStdLibStack,
		isTraceStack,
	)
	for _, option := range options {
		option.apply(opts)
	}

	return opts
}

func (o *opts) filter(s stack.Stack) bool {
	for _, filter := range o.filters {
		if filter(s) {
			return true
		}
	}
	return false
}

func (o *opts) retry(i int) bool {
	if i >= o.maxRetries {
		return false
	}

	d := time.Duration(int(time.Microsecond) << uint(i))
	if d > o.maxSleep {
		d = o.maxSleep
	}
	time.Sleep(d)
	return true
}

// isTestStack is a default filter installed to automatically skip goroutines
// that the testing package runs while the user's tests are running.
func isTestStack(s stack.Stack) bool {
	// Until go1.7, the main goroutine ran RunTests, which started
	// the test in a separate goroutine and waited for that test goroutine
	// to end by waiting on a channel.
	// Since go1.7, a separate goroutine is started to wait for signals.
	// T.Parallel is for parallel tests, which are blocked until all serial
	// tests have run with T.Parallel at the top of the stack.
	// testing.runFuzzTests is for fuzz testing, it's blocked until the test
	// function with all seed corpus have run.
	// testing.runFuzzing is for fuzz testing, it's blocked until a failing
	// input is found.
	switch s.FirstFunction() {
	case "testing.RunTests", "testing.(*T).Run", "testing.(*T).Parallel", "testing.runFuzzing", "testing.runFuzzTests":
		// In pre1.7 and post-1.7, background goroutines started by the testing
		// package are blocked waiting on a channel.
		return strings.HasPrefix(s.State(), "chan receive")
	}
	return false
}

func isSyscallStack(s stack.Stack) bool {
	// Typically runs in the background when code uses CGo:
	// https://github.com/golang/go/issues/16714
	return s.FirstFunction() == "runtime.goexit" && strings.HasPrefix(s.State(), "syscall")
}

func isStdLibStack(s stack.Stack) bool {
	// Importing os/signal starts a background goroutine.
	// The name of the function at the top has changed between versions.
	if f := s.FirstFunction(); f == "os/signal.signal_recv" || f == "os/signal.loop" {
		return true
	}

	// Using signal.Notify will start a runtime goroutine.
	return strings.Contains(s.Full(), "runtime.ensureSigM")
}
