package machine

import "time"

const (
	SchemaVersion = 1
	ListToday     = "today"

	OpTaskCreate          = "task.create"
	OpTaskUpdate          = "task.update"
	OpTaskDelete          = "task.delete"
	OpTaskSetCompleted    = "task.setCompleted"
	OpTaskAddToToday      = "task.addToToday"
	OpTaskRemoveFromToday = "task.removeFromToday"
	OpSubtaskCreate       = "subtask.create"
	OpSubtaskUpdate       = "subtask.update"
	OpSubtaskDelete       = "subtask.delete"
	OpSubtaskSetCompleted = "subtask.setCompleted"
	OpListCreate          = "list.create"
	OpListRename          = "list.rename"
	OpListSetCurrent      = "list.setCurrent"
	OpListDelete          = "list.delete"
)

type Snapshot struct {
	SchemaVersion  int        `json:"schemaVersion"`
	GeneratedAt    time.Time  `json:"generatedAt"`
	Day            string     `json:"day"`
	Revision       string     `json:"revision"`
	PendingCount   int        `json:"pendingCount"`
	CompletedCount int        `json:"completedCount"`
	Tasks          []TaskItem `json:"tasks"`
	Error          string     `json:"error,omitempty"`
}

type TaskItem struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Completed        bool       `json:"completed"`
	Description      string     `json:"description,omitempty"`
	DueDate          string     `json:"dueDate,omitempty"`
	StartTime        string     `json:"startTime,omitempty"`
	EstimateSeconds  int        `json:"estimateSeconds,omitempty"`
	RecurrenceDays   int        `json:"recurrenceDays,omitempty"`
	OwnerList        string     `json:"ownerList,omitempty"`
	SourceList       string     `json:"sourceList,omitempty"`
	Placement        string     `json:"placement,omitempty"`
	Subtasks         []TaskItem `json:"subtasks"`
	AvailableActions []string   `json:"availableActions"`
}

type Request struct {
	SchemaVersion    int         `json:"schemaVersion"`
	RequestID        string      `json:"requestId"`
	Operation        string      `json:"operation"`
	TaskID           string      `json:"taskId,omitempty"`
	ExpectedRevision string      `json:"expectedRevision,omitempty"`
	List             string      `json:"list,omitempty"`
	NewList          string      `json:"newList,omitempty"`
	Changes          TaskChanges `json:"changes"`
}

type TaskChanges struct {
	Title           *string `json:"title,omitempty"`
	Description     *string `json:"description,omitempty"`
	Completed       *bool   `json:"completed,omitempty"`
	DueDate         *string `json:"dueDate,omitempty"`
	StartTime       *string `json:"startTime,omitempty"`
	EstimateSeconds *int    `json:"estimateSeconds,omitempty"`
	RecurrenceDays  *int    `json:"recurrenceDays,omitempty"`
	ParentID        *string `json:"parentId,omitempty"`
}

type ListsSnapshot struct {
	SchemaVersion int      `json:"schemaVersion"`
	Revision      string   `json:"revision"`
	CurrentList   string   `json:"currentList"`
	Lists         []string `json:"lists"`
}

type Response struct {
	Success  bool      `json:"success"`
	Snapshot *Snapshot `json:"snapshot,omitempty"`
	Error    *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
