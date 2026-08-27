// Copyright (c) 2017 Uber Technologies, Inc.

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
// +build tinygo

// goleak uses parts of go that are not yet supported by tinygo.
// This file provides a placeholder to allow programs using goleak
// to compile unchanged with tinygo before such support arrives.

package goleak

// TestingT is the minimal subset of testing.TB that we use.
type TestingT interface {
	Error(...interface{})
}

// VerifyNone marks the given TestingT as failed if any extra goroutines are
// found by Find. This is a helper method to make it easier to integrate in
// tests by doing:
// 	defer VerifyNone(t)
func VerifyNone(t TestingT, options ...Option) {
	// Stub until ported to tinygo
}
