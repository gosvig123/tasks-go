package storage

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/krisitan/tasks-go/internal/task"
)

func revisionBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash[:])
}

func (s *Storage) refreshLoadedRevision(list *task.TaskList) error {
	data, err := os.ReadFile(s.ListPath(list.Name))
	if err != nil {
		return err
	}
	list.LoadedRevision = revisionBytes(data)
	return nil
}

func (s *Storage) verifyLoadedRevision(list *task.TaskList) error {
	if list.LoadedRevision == "" {
		return nil
	}
	data, err := os.ReadFile(s.ListPath(list.Name))
	if err != nil {
		return err
	}
	if revisionBytes(data) != list.LoadedRevision {
		return fmt.Errorf("list %s changed; reload and try again", list.Name)
	}
	return nil
}

func (s *Storage) Revision() (string, error) {
	names, err := s.GetAllLists()
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.WriteString(hash, "current\x00"+s.GetCurrentList()+"\x00"); err != nil {
		return "", err
	}
	for _, name := range names {
		if _, err := io.WriteString(hash, name+"\x00"); err != nil {
			return "", err
		}
		data, err := os.ReadFile(s.ListPath(name))
		if err != nil {
			return "", err
		}
		if _, err := hash.Write(data); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}
