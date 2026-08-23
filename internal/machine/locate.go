package machine

import (
	"github.com/krisitan/tasks-go/internal/task"
)

type locatedTask struct {
	List      *task.TaskList
	ListName  string
	Task      *task.Task
	Parent    *task.Task
	Index     int
	SubIndex  int
	IsSubtask bool
}

func (s *Service) locateCanonical(id string) (*locatedTask, error) {
	if id == "" {
		return nil, apiError(CodeBadRequest, "taskId is required")
	}
	names, err := s.Store.GetAllLists()
	if err != nil {
		return nil, err
	}
	for _, name := range appendNonToday(names) {
		found, loadErr := s.locateInList(name, id)
		if loadErr != nil {
			return nil, loadErr
		}
		if found != nil {
			return found, nil
		}
	}
	return nil, apiError(CodeNotFound, "task %s not found", id)
}

func (s *Service) locateInList(name, id string) (*locatedTask, error) {
	list, err := s.Store.LoadList(name)
	if err != nil {
		return nil, err
	}
	for index, item := range list.Tasks {
		if item.ID == id {
			return &locatedTask{List: list, ListName: name, Task: item, Index: index}, nil
		}
		for subIndex, subtask := range item.Subtasks {
			if subtask.ID == id {
				return &locatedTask{List: list, ListName: name, Task: subtask, Parent: item,
					Index: index, SubIndex: subIndex, IsSubtask: true}, nil
			}
		}
	}
	return nil, nil
}

func appendNonToday(names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		if name != ListToday {
			result = append(result, name)
		}
	}
	return append(result, ListToday)
}

func (s *Service) locateToday(id string) (*locatedTask, error) {
	found, err := s.locateInList(ListToday, id)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, apiError(CodeNotFound, "task %s is not in Today", id)
	}
	return found, nil
}
