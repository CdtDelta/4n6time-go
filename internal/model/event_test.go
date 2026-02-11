package model

import "testing"

func TestEventDefaults(t *testing.T) {
	e := Event{}

	if e.ID != 0 {
		t.Errorf("expected ID to be 0, got %d", e.ID)
	}

	if e.Timezone != "" {
		t.Errorf("expected empty Timezone, got %s", e.Timezone)
	}
}
