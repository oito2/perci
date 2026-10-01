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

package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// SyncWriter wraps w so goroutines running concurrently (see RunConcurrent)
// can share it without interleaving bytes mid-line — every ui.Info/Warning/
// Success/Step call does exactly one underlying Write per invocation, so
// serializing at the Write level is enough to keep each line intact; which
// goroutine's line lands first is still unordered, same as any real
// parallel work.
type SyncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewSyncWriter wraps w for concurrent use.
func NewSyncWriter(w io.Writer) *SyncWriter {
	return &SyncWriter{w: w}
}

func (s *SyncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// RunConcurrent calls fn once per item in items, with at most limit calls
// running at the same time, and blocks until every started call has
// returned. Exists for the narrow cases where a batch of independent,
// I/O-bound per-item operations (ex. downloading a font from its own URL,
// installing a Flatpak app at user scope) isn't already forced sequential
// by RunSudoSequence's single-script batching — most privileged batches
// already run as one script and gain nothing from this. Any io.Writer fn
// writes to must be safe for concurrent use (see SyncWriter); any
// slice/map fn appends to or mutates must be protected by the caller
// (ex. a sync.Mutex around the append).
//
// Once ctx is done no further item is started, and ctx.Err() is part of
// the returned error; a panic in fn is recovered and returned as an error
// instead of taking the process down with it.
func RunConcurrent[T any](ctx context.Context, items []T, limit int, fn func(T)) error {
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	for _, item := range items {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(it T) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("erro interno: %v", r))
					mu.Unlock()
				}
			}()
			fn(it)
		}(item)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
