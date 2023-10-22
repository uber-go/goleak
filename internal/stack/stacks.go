// Copyright (c) 2017-2023 Uber Technologies, Inc.
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

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
)

const _defaultBufferSize = 64 * 1024 // 64 KiB

// Stack represents a single Goroutine's stack.
type Stack struct {
	id            int
	state         string
	firstFunction string

	// Full, raw stack trace.
	fullStack string
}

// ID returns the goroutine ID.
func (s Stack) ID() int {
	return s.id
}

// State returns the Goroutine's state.
func (s Stack) State() string {
	return s.state
}

// Full returns the full stack trace for this goroutine.
func (s Stack) Full() string {
	return s.fullStack
}

// FirstFunction returns the name of the first function on the stack.
func (s Stack) FirstFunction() string {
	return s.firstFunction
}

func (s Stack) String() string {
	return fmt.Sprintf(
		"Goroutine %v in state %v, with %v on top of the stack:\n%s",
		s.id, s.state, s.firstFunction, s.Full())
}

func getStacks(all bool) []Stack {
	trace := getStackBuffer(all)
	stacks, err := newStackParser(bytes.NewReader(trace)).Parse()
	if err != nil {
		// Well-formed stack traces should never fail to parse.
		// If they do, it's a bug in this package.
		// Panic so we can fix it.
		panic(fmt.Sprintf("Failed to parse stack trace: %v\n%s", err, trace))
	}
	return stacks
}

type stackParser struct {
	scan   *scanner
	stacks []Stack
	errors []error
}

func newStackParser(r io.Reader) *stackParser {
	return &stackParser{
		scan: newScanner(r),
	}
}

func (p *stackParser) Parse() ([]Stack, error) {
	for p.scan.Scan() {
		line := p.scan.Text()

		// If we see the goroutine header, start a new stack.
		if strings.HasPrefix(line, "goroutine ") {
			stack, err := p.parseStack(line)
			if err != nil {
				p.errors = append(p.errors, err)
			} else {
				p.stacks = append(p.stacks, stack)
			}
		}
	}

	p.errors = append(p.errors, p.scan.Err())
	return p.stacks, errors.Join(p.errors...)
}

// parseStack parses a single stack trace from the given scanner.
// line is the first line of the stack trace, which should look like:
//
//	goroutine 123 [runnable]:
func (p *stackParser) parseStack(line string) (Stack, error) {
	id, state, err := parseGoStackHeader(line)
	if err != nil {
		return Stack{}, fmt.Errorf("parse header: %w", err)
	}

	// Read the rest of the stack trace.
	var (
		firstFunction string
		fullStack     bytes.Buffer
	)
	for p.scan.Scan() {
		line := p.scan.Text()

		if strings.HasPrefix(line, "goroutine ") {
			// If we see the goroutine header,
			// it's the end of this stack.
			// Unscan so the next Scan sees the same line.
			p.scan.Unscan()
			break
		}

		fullStack.WriteString(line)
		fullStack.WriteByte('\n') // scanner trims the newline

		// The first line after the header is the top of the stack.
		if firstFunction == "" {
			firstFunction, err = parseFirstFunc(line)
			if err != nil {
				return Stack{}, fmt.Errorf("extract function: %w", err)
			}
		}
	}

	return Stack{
		id:            id,
		state:         state,
		firstFunction: firstFunction,
		fullStack:     fullStack.String(),
	}, nil
}

// All returns the stacks for all running goroutines.
func All() []Stack {
	return getStacks(true)
}

// Current returns the stack for the current goroutine.
func Current() Stack {
	return getStacks(false)[0]
}

func getStackBuffer(all bool) []byte {
	for i := _defaultBufferSize; ; i *= 2 {
		buf := make([]byte, i)
		if n := runtime.Stack(buf, all); n < i {
			return buf[:n]
		}
	}
}

func parseFirstFunc(line string) (string, error) {
	line = strings.TrimSpace(line)
	if idx := strings.LastIndex(line, "("); idx > 0 {
		return line[:idx], nil
	}
	return "", fmt.Errorf("no function found: %q", line)
}

// parseGoStackHeader parses a stack header that looks like:
// goroutine 643 [runnable]:\n
// And returns the goroutine ID, and the state.
func parseGoStackHeader(line string) (goroutineID int, state string, err error) {
	// The scanner will have already trimmed the "\n",
	// but we'll guard against it just in case.
	//
	// Trimming them separately makes them both optional.
	line = strings.TrimSuffix(strings.TrimSuffix(line, ":"), "\n")
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return 0, "", fmt.Errorf("unexpected format: %q", line)
	}

	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, "", fmt.Errorf("bad goroutine ID %q in line %q", parts[1], line)
	}

	state = strings.TrimSuffix(strings.TrimPrefix(parts[2], "["), "]")
	return id, state, nil
}
