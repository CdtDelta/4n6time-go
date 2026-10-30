package main

import (
	"errors"
	"testing"
	"time"

	"github.com/cdtdelta/4n6time/internal/database"
)

// ccBlackhole is a non-routable address: a connect to it gets no answer, so
// only the connect timeout or a cancel ends the attempt.
const ccBlackhole = "10.255.255.1"

// ccTimeout sets database.PostgresConnectTimeout for one test and clears
// PGCONNECT_TIMEOUT so the default applies.
func ccTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prev := database.PostgresConnectTimeout
	database.PostgresConnectTimeout = d
	t.Cleanup(func() { database.PostgresConnectTimeout = prev })
	t.Setenv("PGCONNECT_TIMEOUT", "")
}

// ccSkipUnlessTimedOut skips when the network refused the address at once
// instead of leaving it unanswered, since then no timeout can be observed.
func ccSkipUnlessTimedOut(t *testing.T, err error) *database.ConnectTimeoutError {
	t.Helper()
	var te *database.ConnectTimeoutError
	if !errors.As(err, &te) {
		t.Skipf("%s did not time out here (the network answered): %v", ccBlackhole, err)
	}
	return te
}

func TestConnectTimeoutFriendlyMessage(t *testing.T) {
	ccTimeout(t, time.Second)
	app := &App{}
	start := time.Now()
	_, err := app.ConnectPostgres(ccBlackhole, "5432", "db", "user", "pw", "disable")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("connect to %s succeeded", ccBlackhole)
	}
	te := ccSkipUnlessTimedOut(t, err)
	if te.Timeout != time.Second {
		t.Errorf("timeout = %s, want 1s", te.Timeout)
	}
	if elapsed > 4*time.Second {
		t.Errorf("connect took %s, want about the 1s timeout", elapsed)
	}
	f := app.GetOpenFailure()
	want := "Could not reach the PostgreSQL server (no response after 1 second)."
	if f == nil || f.Message != want {
		t.Errorf("failure = %+v, want message %q", f, want)
	}
}

// TestConnectTimeoutExplicit checks a user's own timeout wins over the
// default, both in the connection string and from PGCONNECT_TIMEOUT.
func TestConnectTimeoutExplicit(t *testing.T) {
	ccTimeout(t, 30*time.Second)
	t.Run("connect_timeout in the connection string", func(t *testing.T) {
		start := time.Now()
		_, err := database.OpenPostgres("postgres://user:pw@" + ccBlackhole + ":5432/db?sslmode=disable&connect_timeout=1")
		if err == nil {
			t.Fatalf("connect succeeded")
		}
		if te := ccSkipUnlessTimedOut(t, err); te.Timeout != time.Second {
			t.Errorf("timeout = %s, want the explicit 1s", te.Timeout)
		}
		if time.Since(start) > 5*time.Second {
			t.Errorf("the explicit timeout was not used")
		}
	})
	t.Run("PGCONNECT_TIMEOUT", func(t *testing.T) {
		t.Setenv("PGCONNECT_TIMEOUT", "1")
		app := &App{}
		_, err := app.ConnectPostgres(ccBlackhole, "5432", "db", "user", "pw", "disable")
		if err == nil {
			t.Fatalf("connect succeeded")
		}
		if te := ccSkipUnlessTimedOut(t, err); te.Timeout != time.Second {
			t.Errorf("timeout = %s, want the explicit 1s", te.Timeout)
		}
	})
}

// TestConnectCancel cancels a connect and a create in progress: each returns
// nil, nil, the current database stays open and current, and no failure is
// recorded.
func TestConnectCancel(t *testing.T) {
	ccTimeout(t, 30*time.Second)
	firstPath := currentDatabase(t)
	app := &App{}
	if _, err := app.loadDatabase(firstPath); err != nil {
		t.Fatal(err)
	}
	first := app.store
	defer first.Close()

	calls := map[string]func() (*DBInfo, error){
		"connect": func() (*DBInfo, error) {
			return app.ConnectPostgres(ccBlackhole, "5432", "db", "user", "pw", "disable")
		},
		"create": func() (*DBInfo, error) {
			return app.CreatePostgresDatabase(ccBlackhole, "5432", "db", "user", "pw", "disable")
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			type result struct {
				info *DBInfo
				err  error
			}
			done := make(chan result, 1)
			go func() {
				info, err := call()
				done <- result{info, err}
			}()

			// Wait until the attempt holds the connect slot.
			deadline := time.Now().Add(5 * time.Second)
			for {
				select {
				case r := <-done:
					t.Skipf("the attempt ended before it could be cancelled (the network answered): %v", r.err)
				default:
				}
				app.connectReq.mu.Lock()
				inFlight := app.connectReq.cancel != nil
				app.connectReq.mu.Unlock()
				if inFlight {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("the attempt never started")
				}
				time.Sleep(10 * time.Millisecond)
			}

			app.CancelPostgresConnect()
			select {
			case r := <-done:
				if r.info != nil || r.err != nil {
					t.Errorf("cancelled %s = %+v, %v; want nil, nil", name, r.info, r.err)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("cancel did not end the attempt")
			}
			if app.store != first || app.driver != "sqlite" {
				t.Errorf("store switched after a cancelled %s", name)
			}
			if f := app.GetOpenFailure(); f != nil {
				t.Errorf("failure recorded for a cancel: %+v", f)
			}
			if _, err := app.store.CountEvents("", nil); err != nil {
				t.Errorf("current database unusable: %v", err)
			}
		})
	}
}
