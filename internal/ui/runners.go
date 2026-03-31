package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/krisitan/tasks-go/internal/storage"
)

func RunTaskList(store *storage.Storage) (RunTaskListResult, error) {
	model := NewTaskViewModel(store, ViewSingleList)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return ResultQuit, err
	}
	if m, ok := finalModel.(*TaskViewModel); ok && m.wantsSwitchList {
		return ResultSwitchList, nil
	}
	return ResultQuit, nil
}

func RunAllTasks(store *storage.Storage) error {
	model := NewTaskViewModel(store, ViewAllPending)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func RunAddToday(store *storage.Storage) (int, error) {
	model := NewTaskViewModel(store, ViewPicker)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return 0, err
	}
	if m, ok := finalModel.(*TaskViewModel); ok {
		return m.Added(), nil
	}
	return 0, nil
}
