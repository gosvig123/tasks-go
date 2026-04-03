package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SelectedTask identifies a task by its list and content.
type SelectedTask struct {
	List            string     `json:"list"`
	Content         string     `json:"content"`
	Source          string     `json:"source,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	AccumulatedSecs int        `json:"accumulated_secs,omitempty"`
}

// Elapsed returns the total tracked duration.
func (st *SelectedTask) Elapsed() time.Duration {
	d := time.Duration(st.AccumulatedSecs) * time.Second
	if st.StartedAt != nil {
		d += time.Since(*st.StartedAt)
	}
	return d
}

// IsRunning reports whether the timer is active.
func (st *SelectedTask) IsRunning() bool {
	return st.StartedAt != nil
}

// LocalState holds ephemeral state that is stored locally and not synced.
type LocalState struct {
	SelectedTask *SelectedTask `json:"selected_task,omitempty"`
}

// LoadLocalState reads the local state from disk.
// Returns an empty state if the file does not exist.
func (s *Storage) LoadLocalState() (*LocalState, error) {
	data, err := os.ReadFile(s.LocalStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &LocalState{}, nil
		}
		return nil, fmt.Errorf("reading local state: %w", err)
	}

	var state LocalState
	if err := json.Unmarshal(data, &state); err != nil {
		return &LocalState{}, nil
	}
	return &state, nil
}

// SaveLocalState writes the local state to disk.
func (s *Storage) SaveLocalState(state *LocalState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling local state: %w", err)
	}
	return os.WriteFile(s.LocalStateFile, data, 0644)
}

// GetSelectedTask returns the currently selected task, or nil if none.
func (s *Storage) GetSelectedTask() (*SelectedTask, error) {
	state, err := s.LoadLocalState()
	if err != nil {
		return nil, err
	}
	return state.SelectedTask, nil
}

// SetSelectedTask sets the selected task. Pass nil to clear.
func (s *Storage) SetSelectedTask(sel *SelectedTask) error {
	state, err := s.LoadLocalState()
	if err != nil {
		state = &LocalState{}
	}
	state.SelectedTask = sel
	return s.SaveLocalState(state)
}

// ClearSelectedTask removes the selected task.
func (s *Storage) ClearSelectedTask() error {
	return s.SetSelectedTask(nil)
}
