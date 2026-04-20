package handlers

import (
	"time"

	rrdomain "example.com/taskservice/internal/domain/recurrencerule"
	taskdomain "example.com/taskservice/internal/domain/task"
)

type taskMutationDTO struct {
	Title          string                  `json:"title"`
	Description    string                  `json:"description"`
	Status         taskdomain.Status       `json:"status"`
	IsRecurring    bool                    `json:"is_recurring"`
	RecurrenceRule rrdomain.RecurrenceRule `json:"recurrence_rule"`
}

type taskDTO struct {
	ID             int64                   `json:"id"`
	Title          string                  `json:"title"`
	Description    string                  `json:"description"`
	Status         taskdomain.Status       `json:"status"`
	IsRecurring    bool                    `json:"is_recurring"`
	RecurrenceRule rrdomain.RecurrenceRule `json:"recurrence_rule"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	return taskDTO{
		ID:             task.ID,
		Title:          task.Title,
		Description:    task.Description,
		Status:         task.Status,
		IsRecurring:    task.IsRecurring,
		RecurrenceRule: task.RecurrenceRule,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
	}
}
