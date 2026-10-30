package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cdtdelta/4n6time/internal/database"
)

// blockingStore blocks the next count or histogram call, once armed, until
// its context is cancelled, and closes started when it begins to block.
// Other calls pass through.
type blockingStore struct {
	database.Store
	mu      sync.Mutex
	armed   bool
	started chan struct{}
}

func (s *blockingStore) arm() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
	s.started = make(chan struct{})
	return s.started
}

// take disarms the store and reports whether this call should block.
func (s *blockingStore) take() (chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.armed {
		return nil, false
	}
	s.armed = false
	return s.started, true
}

func (s *blockingStore) ExecuteCountQueryContext(ctx context.Context, q string, args []interface{}) (int64, error) {
	if started, ok := s.take(); ok {
		close(started)
		<-ctx.Done()
		return 0, ctx.Err()
	}
	return s.Store.ExecuteCountQueryContext(ctx, q, args)
}

func (s *blockingStore) GetTimelineHistogramContext(ctx context.Context, where string, args []interface{}, w *database.HistogramWindow) ([]database.TimelineBucket, error) {
	if started, ok := s.take(); ok {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return s.Store.GetTimelineHistogramContext(ctx, where, args, w)
}

func rsWait(t *testing.T, ch chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("the first request never reached the store")
	}
}

func rsApp(t *testing.T) (*App, *blockingStore) {
	t.Helper()
	store := newSQLiteForPush(t, "requests.db")
	if _, err := store.InsertEvents(ccEvents(3, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	bs := &blockingStore{Store: store}
	return &App{store: bs, driver: "sqlite"}, bs
}

// TestGridRequestSuperseded starts a grid request that blocks, then a second
// one: the second cancels the first, which returns errSuperseded, and the
// second succeeds. AdvancedSearch shares the slot.
func TestGridRequestSuperseded(t *testing.T) {
	app, bs := rsApp(t)
	second := map[string]func() (*QueryResponse, error){
		"QueryEvents":    func() (*QueryResponse, error) { return app.QueryEvents(QueryRequest{}) },
		"AdvancedSearch": func() (*QueryResponse, error) { return app.AdvancedSearch("host = 'WS1'", 1, 10) },
	}
	for name, call := range second {
		t.Run(name, func(t *testing.T) {
			started := bs.arm()
			first := make(chan error, 1)
			go func() {
				_, err := app.QueryEvents(QueryRequest{})
				first <- err
			}()
			rsWait(t, started)

			resp, err := call()
			if err != nil || resp.TotalCount != 3 {
				t.Fatalf("second request = %+v, %v; want 3 events", resp, err)
			}
			select {
			case err := <-first:
				if !errors.Is(err, errSuperseded) || err.Error() != supersededText {
					t.Errorf("first request error = %v, want errSuperseded", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("first request was not cancelled")
			}
		})
	}
}

func TestHistogramRequestSuperseded(t *testing.T) {
	app, bs := rsApp(t)
	started := bs.arm()
	first := make(chan error, 1)
	go func() {
		_, err := app.GetTimelineHistogram(QueryRequest{})
		first <- err
	}()
	rsWait(t, started)

	buckets, err := app.GetTimelineHistogram(QueryRequest{})
	if err != nil || len(buckets) == 0 {
		t.Fatalf("second histogram = %v, %v; want buckets", buckets, err)
	}
	select {
	case err := <-first:
		if !errors.Is(err, errSuperseded) {
			t.Errorf("first histogram error = %v, want errSuperseded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("first histogram was not cancelled")
	}
}

// TestSupersededTextMatchesFrontend keeps the frontend's match in step.
func TestSupersededTextMatchesFrontend(t *testing.T) {
	src, err := os.ReadFile("frontend/src/requestErrors.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "'"+supersededText+"'") {
		t.Errorf("frontend/src/requestErrors.js does not match %q", supersededText)
	}
}

// rsSleepMarker tags the test's sleeping query in pg_stat_activity.
const rsSleepMarker = "4n6time-cancel-test"

// sleepStore runs a 30-second pg_sleep in place of the next count, once armed.
type sleepStore struct {
	*blockingStore
}

func (s *sleepStore) ExecuteCountQueryContext(ctx context.Context, q string, args []interface{}) (int64, error) {
	if started, ok := s.take(); ok {
		close(started)
		return s.Store.ExecuteCountQueryContext(ctx, "SELECT /* "+rsSleepMarker+" */ 1::bigint FROM pg_sleep(30)", nil)
	}
	return s.Store.ExecuteCountQueryContext(ctx, q, args)
}

// rsActiveSleeps counts server sessions still running the marked query.
func rsActiveSleeps(t *testing.T, mon *sql.DB) int {
	t.Helper()
	var n int
	err := mon.QueryRow(`SELECT count(*) FROM pg_stat_activity
		WHERE state = 'active' AND pid <> pg_backend_pid() AND position($1 in query) > 0`, rsSleepMarker).Scan(&n)
	if err != nil {
		t.Fatalf("reading pg_stat_activity: %v", err)
	}
	return n
}

// TestGridCancelStopsPostgresQuery checks that a superseded grid request
// stops its query on the server, not just in the client.
func TestGridCancelStopsPostgresQuery(t *testing.T) {
	connStr := mxCreatePGDatabase(t)
	pg, err := database.CreatePostgres(connStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Close() })
	if _, err := pg.InsertEvents(ccEvents(3, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	mon, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mon.Close() })

	ss := &sleepStore{&blockingStore{Store: pg}}
	app := &App{store: ss, driver: "postgres"}
	started := ss.arm()
	first := make(chan error, 1)
	go func() {
		_, err := app.QueryEvents(QueryRequest{})
		first <- err
	}()
	rsWait(t, started)
	deadline := time.Now().Add(5 * time.Second)
	for rsActiveSleeps(t, mon) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the sleeping query never became active on the server")
		}
		time.Sleep(50 * time.Millisecond)
	}

	if resp, err := app.QueryEvents(QueryRequest{}); err != nil || resp.TotalCount != 3 {
		t.Fatalf("second request = %+v, %v", resp, err)
	}
	select {
	case err := <-first:
		if !errors.Is(err, errSuperseded) {
			t.Errorf("first request error = %v, want errSuperseded", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("first request was not cancelled")
	}
	deadline = time.Now().Add(5 * time.Second)
	for rsActiveSleeps(t, mon) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the superseded query is still running on the server")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
