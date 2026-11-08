package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Activity struct {
	T      time.Time `json:"t"`
	IP     string    `json:"ip"`
	Action string    `json:"action"`
	Detail string    `json:"detail,omitempty"`
	OK     bool      `json:"ok"`
}

type ActivityStore struct {
	mu    sync.Mutex
	path  string
	items []Activity
}

func newActivityStore(home string) *ActivityStore {
	s := &ActivityStore{path: filepath.Join(home, "activity.jsonl")}
	s.load()
	return s
}

func (s *ActivityStore) load() {
	f, err := os.Open(s.path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 256*1024)
	for sc.Scan() {
		var a Activity
		if json.Unmarshal(sc.Bytes(), &a) == nil {
			s.items = append(s.items, a)
		}
	}
	if len(s.items) > 500 {
		s.items = s.items[len(s.items)-500:]
	}
}

func (s *ActivityStore) Add(a Activity) {
	if a.T.IsZero() {
		a.T = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, a)
	if len(s.items) > 500 {
		s.items = s.items[len(s.items)-500:]
	}
	b, err := json.Marshal(a)
	if err != nil {
		return
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
	// Keep the file bounded too.
	if fi, err := os.Stat(s.path); err == nil && fi.Size() > 512*1024 {
		s.rewriteLocked()
	}
}

func (s *ActivityStore) rewriteLocked() {
	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	w := bufio.NewWriter(f)
	start := 0
	if len(s.items) > 400 {
		start = len(s.items) - 400
	}
	for _, a := range s.items[start:] {
		b, err := json.Marshal(a)
		if err != nil {
			continue
		}
		_, _ = w.Write(append(b, '\n'))
	}
	_ = w.Flush()
	_ = f.Close()
	_ = os.Rename(tmp, s.path)
}

// Recent returns the newest activities first.
func (s *ActivityStore) Recent(n int) []Activity {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 || n > len(s.items) {
		n = len(s.items)
	}
	out := make([]Activity, 0, n)
	for i := len(s.items) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, s.items[i])
	}
	return out
}
