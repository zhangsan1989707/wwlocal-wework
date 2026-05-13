package service

import (
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type OperationLogService struct {
	repo *repository.OperationLogRepository
}

func NewOperationLogService(repo *repository.OperationLogRepository) *OperationLogService {
	return &OperationLogService{repo: repo}
}

func (s *OperationLogService) Save(entry *model.OperationLog) error {
	return s.repo.Create(entry)
}

func (s *OperationLogService) List(page, pageSize int, action string, statusCode int) ([]model.OperationLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return s.repo.List(page, pageSize, action, statusCode)
}

func (s *OperationLogService) GetDistinctActions() ([]string, error) {
	return s.repo.GetDistinctActions()
}
