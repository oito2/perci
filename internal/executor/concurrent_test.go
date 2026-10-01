// Copyright (C) 2026  oito2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package executor_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func TestRunConcurrent_CallsEveryItem(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	var sum atomic.Int64

	_ = executor.RunConcurrent(context.Background(), items, 3, func(n int) {
		sum.Add(int64(n))
	})

	if got, want := sum.Load(), int64(55); got != want {
		t.Errorf("sum = %d, want %d", got, want)
	}
}

func TestRunConcurrent_EmptyItems(t *testing.T) {
	called := false
	_ = executor.RunConcurrent(context.Background(), []int{}, 4, func(int) { called = true })
	if called {
		t.Error("fn should not be called for an empty item list")
	}
}

func TestRunConcurrent_ZeroLimitStillRuns(t *testing.T) {
	// limit < 1 must not deadlock (an unbuffered/zero-size semaphore channel
	// would never let any goroutine acquire it) — falls back to limit=1.
	items := []int{1, 2, 3}
	var count atomic.Int64
	_ = executor.RunConcurrent(context.Background(), items, 0, func(int) { count.Add(1) })
	if got := count.Load(); got != 3 {
		t.Errorf("count = %d, want 3", got)
	}
}

func TestSyncWriter_SerializesWrites(t *testing.T) {
	var buf bytes.Buffer
	w := executor.NewSyncWriter(&buf)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = strings.Repeat("x", 20) + "\n"
	}

	_ = executor.RunConcurrent(context.Background(), lines, 8, func(line string) {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Errorf("Write: %v", err)
		}
	})

	got := buf.String()
	for _, l := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if l != strings.Repeat("x", 20) {
			t.Fatalf("a line was corrupted by concurrent writes: %q", l)
		}
	}
	if want := 50; strings.Count(got, "\n") != want {
		t.Errorf("got %d lines, want %d (some output was dropped or merged)", strings.Count(got, "\n"), want)
	}
}

// A panic in fn comes back as an error; the other items still run.
func TestRunConcurrent_RecoversPanic(t *testing.T) {
	var count atomic.Int64
	err := executor.RunConcurrent(context.Background(), []int{1, 2, 3}, 2, func(n int) {
		if n == 2 {
			panic("boom")
		}
		count.Add(1)
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want the panic", err)
	}
	if count.Load() != 2 {
		t.Errorf("ran %d non-panicking items, want 2", count.Load())
	}
}

// A cancelled ctx starts nothing and is reported.
func TestRunConcurrent_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := executor.RunConcurrent(ctx, []int{1, 2}, 1, func(int) { called = true })
	if called || !errors.Is(err, context.Canceled) {
		t.Errorf("called=%v err=%v", called, err)
	}
}
