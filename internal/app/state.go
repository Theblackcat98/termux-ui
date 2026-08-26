package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// State persists across runs in ~/.cache/termux-ui/state.json: pending
// restart-required changes, last update-check timestamp and the cached
// upstream version.
type State struct {
	RestartPending  map[string]time.Time `json:"restart_pending"` // property -> changed at
	LastUpdateCheck time.Time            `json:"last_update_check"`
	LatestVersion   string               `json:"latest_version,omitempty"`
}

// StatePath is where the TUI keeps its state file.
func StatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "termux-ui", "state.json")
}

// LoadState reads the state file; missing or corrupt files yield fresh state.
func LoadState() *State {
	s := &State{RestartPending: map[string]time.Time{}}
	data, err := os.ReadFile(StatePath())
	if err == nil {
		if err := json.Unmarshal(data, s); err != nil || s.RestartPending == nil {
			s = &State{RestartPending: map[string]time.Time{}}
		}
	}
	return s
}

// Save writes the state atomically.
func (s *State) Save() error {
	path := StatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// MarkRestart records that a restart-required property changed.
func (s *State) MarkRestart(prop string) {
	if s.RestartPending == nil {
		s.RestartPending = map[string]time.Time{}
	}
	s.RestartPending[prop] = time.Now()
}

// ClearRestart removes a satisfied restart entry.
func (s *State) ClearRestart(prop string) {
	delete(s.RestartPending, prop)
}

// RestartList returns pending properties oldest-first.
func (s *State) RestartList() []string {
	type kv struct {
		k string
		t time.Time
	}
	var kvs []kv
	for k, t := range s.RestartPending {
		kvs = append(kvs, kv{k, t})
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].t.Before(kvs[j].t) })
	out := make([]string, len(kvs))
	for i, v := range kvs {
		out[i] = v.k
	}
	return out
}
