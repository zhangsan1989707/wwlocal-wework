package handler

import (
	"net/http"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"

	"github.com/labstack/echo/v4"
)

type TaskHandler struct {
	taskQueueSvc *service.TaskQueueService
}

func NewTaskHandler(taskQueueSvc *service.TaskQueueService) *TaskHandler {
	return &TaskHandler{taskQueueSvc: taskQueueSvc}
}

type SubmitTaskRequest struct {
	Type       model.TaskType `json:"type"`
	FeatureIDs []int          `json:"feature_ids"`
	StartTime  int64          `json:"start_time"`
	EndTime    int64          `json:"end_time"`
}

func (h *TaskHandler) SubmitTask(c echo.Context) error {
	if !h.taskQueueSvc.IsEnabled() {
		return response.Error(c, http.StatusServiceUnavailable, "task queue is disabled")
	}

	req := new(SubmitTaskRequest)
	if err := c.Bind(req); err != nil {
		return response.ValidationError(c, err)
	}

	task := &model.SyncTask{
		Type:       req.Type,
		FeatureIDs: req.FeatureIDs,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
	}

	taskID, err := h.taskQueueSvc.SubmitTask(task)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, err.Error())
	}

	return response.Success(c, map[string]interface{}{"task_id": taskID})
}

func (h *TaskHandler) GetTask(c echo.Context) error {
	if !h.taskQueueSvc.IsEnabled() {
		return response.Error(c, http.StatusServiceUnavailable, "task queue is disabled")
	}

	taskID := c.Param("id")
	task, err := h.taskQueueSvc.GetTask(taskID)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "task not found")
	}

	return response.Success(c, task)
}

func (h *TaskHandler) ListTasks(c echo.Context) error {
	if !h.taskQueueSvc.IsEnabled() {
		return response.Error(c, http.StatusServiceUnavailable, "task queue is disabled")
	}

	tasks, err := h.taskQueueSvc.ListTasks(50)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, err.Error())
	}

	return response.Success(c, tasks)
}

func (h *TaskHandler) CancelTask(c echo.Context) error {
	if !h.taskQueueSvc.IsEnabled() {
		return response.Error(c, http.StatusServiceUnavailable, "task queue is disabled")
	}

	taskID := c.Param("id")
	if err := h.taskQueueSvc.CancelTask(taskID); err != nil {
		return response.Error(c, http.StatusBadRequest, err.Error())
	}

	return response.Success(c, nil)
}

func (h *TaskHandler) RetryTask(c echo.Context) error {
	if !h.taskQueueSvc.IsEnabled() {
		return response.Error(c, http.StatusServiceUnavailable, "task queue is disabled")
	}

	taskID := c.Param("id")
	if err := h.taskQueueSvc.RetryTask(taskID); err != nil {
		return response.Error(c, http.StatusBadRequest, err.Error())
	}

	return response.Success(c, nil)
}
