package main

import (
	"context"
	"errors"
	"sync"
)

// supersededText is the error text of a request cancelled because a newer
// one of the same kind started. The frontend matches it (isSuperseded in
// App.jsx and TimelineChart.jsx) and drops the result silently.
const supersededText = "request superseded by a newer request"

// errSuperseded is returned by a grid or histogram request whose context was
// cancelled by a newer request of the same kind.
var errSuperseded = errors.New(supersededText)

// requestSlot holds the cancel function of the one in-flight request of a
// kind (grid, histogram, PostgreSQL connect). Starting a request cancels the
// previous one, so a superseded query stops: SQLite interrupts the statement,
// and PostgreSQL gets a cancel request (database.openPGDB).
type requestSlot struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	gen    uint64
}

// begin cancels the request in flight, if any, and starts a new one. The
// returned done must be called when the request finishes; it frees the slot
// if no newer request has taken it, and releases the context.
func (s *requestSlot) begin() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.gen++
	gen := s.gen
	s.cancel = cancel
	s.mu.Unlock()
	return ctx, func() {
		s.mu.Lock()
		if s.gen == gen {
			s.cancel = nil
		}
		s.mu.Unlock()
		cancel()
	}
}

// cancelCurrent cancels the request in flight, if any.
func (s *requestSlot) cancelCurrent() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.mu.Unlock()
}

// supersededOr returns errSuperseded when ctx was cancelled, and err
// otherwise. A store error after a cancel is the cancel, whatever its text.
func supersededOr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return errSuperseded
	}
	return err
}
