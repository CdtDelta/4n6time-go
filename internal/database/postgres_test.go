package database

import (
	"net/url"
	"strings"
	"testing"
)

// These tests exercise MaskConnStr only; they do not need a PostgreSQL server.

func TestMaskConnStrPasswordMasked(t *testing.T) {
	in := "postgres://analyst:s3cret@db.example.local:5432/cases?sslmode=require"
	got := MaskConnStr(in)

	if strings.Contains(got, "s3cret") {
		t.Errorf("MaskConnStr leaked password: %q", got)
	}
	want := "postgres://analyst@db.example.local:5432/cases?sslmode=require"
	if got != want {
		t.Errorf("MaskConnStr = %q, want %q", got, want)
	}
}

func TestMaskConnStrSpecialCharPasswordMasked(t *testing.T) {
	// Built the same way app.go builds connection strings, so special
	// characters are percent-encoded.
	password := "p@ss:w#rd/%1"
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword("analyst", password),
		Host:     "db.example.local:5432",
		Path:     "/cases",
		RawQuery: "sslmode=disable",
	}
	got := MaskConnStr(u.String())

	if strings.Contains(got, password) || strings.Contains(got, url.QueryEscape(password)) {
		t.Errorf("MaskConnStr leaked password: %q", got)
	}
	for _, fragment := range []string{"ss:w", "w#rd", "rd/%1", "%40ss", "w%23rd"} {
		if strings.Contains(got, fragment) {
			t.Errorf("MaskConnStr output %q contains password fragment %q", got, fragment)
		}
	}
	want := "postgres://analyst@db.example.local:5432/cases?sslmode=disable"
	if got != want {
		t.Errorf("MaskConnStr = %q, want %q", got, want)
	}
}

func TestMaskConnStrUnparseableFallsBack(t *testing.T) {
	// A non-numeric port makes url.Parse fail, forcing the fallback path.
	in := "postgres://analyst:s3cret@db.example.local:badport/cases"
	if _, err := url.Parse(in); err == nil {
		t.Fatalf("test input unexpectedly parsed; fallback path not exercised")
	}

	got := MaskConnStr(in)
	if strings.Contains(got, "s3cret") {
		t.Errorf("MaskConnStr leaked password: %q", got)
	}
	want := "postgres://analyst:****@db.example.local:badport/cases"
	if got != want {
		t.Errorf("MaskConnStr = %q, want %q", got, want)
	}
}

func TestMaskConnStrNoCredentialsUnchanged(t *testing.T) {
	tests := []string{
		"postgres://db.example.local:5432/cases?sslmode=disable",
		"postgres://analyst@db.example.local:5432/cases",
		"/home/analyst/cases/timeline.db",
	}
	for _, in := range tests {
		if got := MaskConnStr(in); got != in {
			t.Errorf("MaskConnStr(%q) = %q, want unchanged", in, got)
		}
	}
}
