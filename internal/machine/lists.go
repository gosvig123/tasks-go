package machine

func (s *Service) Lists() (*ListsSnapshot, error) {
	var result *ListsSnapshot
	err := s.Store.WithLock(func() error {
		if _, err := s.Store.ResetTodayList(); err != nil {
			return err
		}
		lists, err := s.Store.GetAllLists()
		if err != nil {
			return err
		}
		if lists == nil {
			lists = make([]string, 0)
		}
		revision, err := s.Store.Revision()
		if err != nil {
			return err
		}
		result = &ListsSnapshot{SchemaVersion: SchemaVersion, Revision: revision,
			CurrentList: currentList(s.Store.GetCurrentList(), lists), Lists: lists}
		return nil
	})
	return result, err
}

func currentList(configured string, lists []string) string {
	fallback := ""
	for _, name := range lists {
		if name == configured {
			return configured
		}
		if fallback == "" || name == ListToday {
			fallback = name
		}
	}
	return fallback
}

func (s *Service) mutateList(request Request) error {
	if err := validateListRequest(request); err != nil {
		return err
	}
	switch request.Operation {
	case OpListCreate:
		return s.createList(request.List)
	case OpListRename:
		return s.renameList(request.List, request.NewList)
	case OpListSetCurrent:
		return s.setCurrentList(request.List)
	case OpListDelete:
		return s.deleteList(request.List)
	}
	return apiError(CodeBadRequest, "unsupported list operation %q", request.Operation)
}

func validateListRequest(request Request) error {
	if err := validateListName(request.List); err != nil {
		return err
	}
	if request.Operation == OpListCreate {
		return validateNewListName(request.List)
	}
	if request.Operation == OpListRename {
		if err := validateNewListName(request.NewList); err != nil {
			return err
		}
	}
	if request.List == ListToday && isReservedListOperation(request.Operation) {
		return apiError(CodeBadRequest, "the Today list cannot be renamed or deleted")
	}
	if request.Operation == OpListRename && request.NewList == ListToday {
		return apiError(CodeBadRequest, "a list cannot be renamed to Today")
	}
	return nil
}

func isReservedListOperation(operation string) bool {
	return operation == OpListRename || operation == OpListDelete
}

func (s *Service) createList(name string) error {
	if s.Store.ListExists(name) {
		return apiError(CodeBadRequest, "list %q already exists", name)
	}
	return s.Store.CreateList(name)
}

func (s *Service) renameList(oldName, newName string) error {
	if !s.Store.ListExists(oldName) {
		return apiError(CodeNotFound, "list %q not found", oldName)
	}
	if s.Store.ListExists(newName) {
		return apiError(CodeBadRequest, "list %q already exists", newName)
	}
	if err := s.Store.RenameList(oldName, newName); err != nil {
		return err
	}
	if s.Store.GetCurrentList() == oldName {
		return s.Store.SetCurrentList(newName)
	}
	return nil
}

func (s *Service) setCurrentList(name string) error {
	if !s.Store.ListExists(name) {
		return apiError(CodeNotFound, "list %q not found", name)
	}
	return s.Store.SetCurrentList(name)
}

func (s *Service) deleteList(name string) error {
	if !s.Store.ListExists(name) {
		return apiError(CodeNotFound, "list %q not found", name)
	}
	if s.Store.GetCurrentList() == name {
		return apiError(CodeBadRequest, "cannot delete the current list")
	}
	return s.Store.DeleteList(name)
}
