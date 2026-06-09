package handler

import (
	"fmt"
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
		return response.Error(c, 400, "invalid request body")
	}
	if err := validateSubmitTaskRequest(req); err != nil {
		return response.Error(c, http.StatusBadRequest, err.Error())
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

func validateSubmitTaskRequest(req *SubmitTaskRequest) error {
	switch req.Type {
	case model.TaskTypeLogSync, model.TaskTypeContactSync, model.TaskTypeAdminLogSync:
	default:
		return fmt.Errorf("type must be one of log_sync, contact_sync, admin_log_sync")
	}

	for _, id := range req.FeatureIDs {
		if id <= 0 {
			return fmt.Errorf("feature_ids must contain positive integers")
		}
	}
	if len(req.FeatureIDs) > 0 && req.Type != model.TaskTypeLogSync {
		return fmt.Errorf("feature_ids can only be used with log_sync tasks")
	}

	if req.StartTime < 0 || req.EndTime < 0 {
		return fmt.Errorf("start_time and end_time must be non-negative")
	}
	if req.StartTime > 0 || req.EndTime > 0 {
		if req.StartTime <= 0 || req.EndTime <= 0 {
			return fmt.Errorf("start_time and end_time must be provided together")
		}
		if req.EndTime < req.StartTime {
			return fmt.Errorf("end_time must be greater than or equal to start_time")
		}
	}

	return nil
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
