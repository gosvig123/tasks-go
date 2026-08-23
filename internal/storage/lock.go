package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/krisitan/tasks-go/internal/task"
)

// WithLock serializes a complete task storage transaction.
func (s *Storage) WithLock(action func() error) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	file, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("opening task lock: %w", err)
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("locking task storage: %w", err)
	}
	defer unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return action()
}

func (s *Storage) ResetTodayListLocked() (int, error) {
	added := 0
	err := s.WithLock(func() error {
		var resetErr error
		added, resetErr = s.ResetTodayList()
		return resetErr
	})
	return added, err
}

// SaveListLocked serializes a standalone list write with machine transactions.
func (s *Storage) SaveListLocked(list *task.TaskList) error {
	return s.WithLock(func() error {
		if err := s.verifyLoadedRevision(list); err != nil {
			return err
		}
		return s.SaveList(list)
	})
}

func (s *Storage) lockPath() string {
	if s.LockFile != "" {
		return s.LockFile
	}
	return filepath.Join(s.TasksDir, ".tasks.lock")
}
