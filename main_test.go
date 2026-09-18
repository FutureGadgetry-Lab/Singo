package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseStatName(t *testing.T) {
	u, d, ok := parseStatName("user>>>alice>>>traffic>>>uplink")
	if !ok || u != "alice" || d != "uplink" {
		t.Fatalf("unexpected: %q %q %v", u, d, ok)
	}
	if _, _, ok := parseStatName("inbound>>>main>>>traffic>>>uplink"); ok {
		t.Fatal("accepted non-user stat")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := openStore(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.add(map[string]traffic{"alice": {Up: 10, Down: 20}}, time.Date(2026, 9, 18, 1, 0, 0, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	loaded, err := openStore(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	o := loaded.overview(collectorStatus{})
	if len(o.Users) != 1 || o.Users[0].Total != 30 {
		t.Fatalf("unexpected overview: %+v", o)
	}
}
