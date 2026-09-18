package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type traffic struct{ Up, Down int64 }
type dayData struct {
	Date  string             `json:"date"`
	Users map[string]traffic `json:"users"`
}
type state struct {
	Version   int                `json:"version"`
	Users     map[string]traffic `json:"users"`
	Days      []dayData          `json:"days"`
	UpdatedAt time.Time          `json:"updated_at"`
}
type store struct {
	mu       sync.RWMutex
	path     string
	keepDays int
	data     state
}

func openStore(path string, keepDays int) (*store, error) {
	s := &store{path: path, keepDays: keepDays, data: state{Version: 1, Users: map[string]traffic{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, err
	}
	if s.data.Users == nil {
		s.data.Users = map[string]traffic{}
	}
	return s, nil
}

func (s *store) add(delta map[string]traffic, now time.Time) error {
	s.mu.Lock()
	date := now.Local().Format("2006-01-02")
	if len(s.data.Days) == 0 || s.data.Days[len(s.data.Days)-1].Date != date {
		s.data.Days = append(s.data.Days, dayData{Date: date, Users: map[string]traffic{}})
	}
	day := &s.data.Days[len(s.data.Days)-1]
	for user, d := range delta {
		t := s.data.Users[user]
		t.Up += max64(d.Up, 0)
		t.Down += max64(d.Down, 0)
		s.data.Users[user] = t
		t = day.Users[user]
		t.Up += max64(d.Up, 0)
		t.Down += max64(d.Down, 0)
		day.Users[user] = t
	}
	if excess := len(s.data.Days) - s.keepDays; excess > 0 {
		s.data.Days = append([]dayData(nil), s.data.Days[excess:]...)
	}
	s.data.UpdatedAt = now
	s.mu.Unlock()
	return s.save()
}

func (s *store) save() error {
	s.mu.RLock()
	b, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

type userView struct {
	Name  string `json:"name"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
	Total int64  `json:"total"`
}
type overviewResponse struct {
	Users     []userView      `json:"users"`
	Total     traffic         `json:"total"`
	UpdatedAt time.Time       `json:"updated_at"`
	Collector collectorStatus `json:"collector"`
}

func (s *store) overview(status collectorStatus) overviewResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := overviewResponse{Users: []userView{}, UpdatedAt: s.data.UpdatedAt, Collector: status}
	for name, t := range s.data.Users {
		r.Users = append(r.Users, userView{Name: name, Up: t.Up, Down: t.Down, Total: t.Up + t.Down})
		r.Total.Up += t.Up
		r.Total.Down += t.Down
	}
	sort.Slice(r.Users, func(i, j int) bool { return r.Users[i].Total > r.Users[j].Total })
	return r
}

func (s *store) history() []dayData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data.Days) == 0 {
		return []dayData{}
	}
	b, _ := json.Marshal(s.data.Days)
	var out []dayData
	_ = json.Unmarshal(b, &out)
	return out
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
