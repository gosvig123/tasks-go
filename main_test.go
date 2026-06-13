package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/krisitan/tasks-go/internal/gist"
	"github.com/krisitan/tasks-go/internal/storage"
)

type stubGistClient struct {
	configured bool
	syncErr    error
	syncCalls  int
}

func (c *stubGistClient) IsConfigured() bool {
	return c.configured
}

func (c *stubGistClient) Sync(string) (*gist.Gist, error) {
	c.syncCalls++
	if c.syncErr != nil {
		return nil, c.syncErr
	}
	return &gist.Gist{}, nil
}

func TestDailyRefreshPrunesOnlyAfterSuccessfulGistSync(t *testing.T) {
	tests := []struct {
		name       string
		configured bool
		syncErr    error
		wantPrune  int
		wantSync   int
	}{
		{name: "successful sync triggers prune", configured: true, wantPrune: 1, wantSync: 1},
		{name: "sync failure skips prune", configured: true, syncErr: errors.New("sync failed"), wantSync: 1},
		{name: "disabled sync skips prune", configured: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store = &storage.Storage{
				TasksDir:        dir,
				CurrentListFile: filepath.Join(dir, ".current-list"),
				LastResetFile:   filepath.Join(dir, ".last-reset"),
				LastSyncFile:    filepath.Join(dir, ".last-sync"),
			}

			client := &stubGistClient{configured: tt.configured, syncErr: tt.syncErr}
			oldNewGistClient := newGistClient
			oldPruneCompletedTasks := pruneCompletedTasks
			defer func() {
				newGistClient = oldNewGistClient
				pruneCompletedTasks = oldPruneCompletedTasks
			}()

			pruneCalls := 0
			newGistClient = func() gistSyncClient {
				return client
			}
			pruneCompletedTasks = func() (int, error) {
				pruneCalls++
				return 0, nil
			}

			dailyRefresh()

			if client.syncCalls != tt.wantSync {
				t.Fatalf("expected %d sync calls, got %d", tt.wantSync, client.syncCalls)
			}
			if pruneCalls != tt.wantPrune {
				t.Fatalf("expected %d prune calls, got %d", tt.wantPrune, pruneCalls)
			}
		})
	}
}
