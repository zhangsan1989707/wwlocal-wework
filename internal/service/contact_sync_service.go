package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

const (
	contactDetailBatchSize   = 500
	contactDetailConcurrency = 5
)

type ContactSyncService struct {
	contactSvc      *ContactService
	contactRepo     *repository.ContactRepository
	syncHistoryRepo *repository.SyncHistoryRepository
	mu              sync.Mutex
	status          *ContactSyncStatus
	cancelCh        chan struct{}
}

type ContactSyncStatus struct {
	Running      bool      `json:"running"`
	Phase        string    `json:"phase"`
	Progress     int       `json:"progress"`
	Total        int       `json:"total"`
	NewCount     int       `json:"new_count"`
	UpdatedCount int       `json:"updated_count"`
	DeletedCount int       `json:"deleted_count"`
	FailedCount  int       `json:"failed_count"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
	LastSync     time.Time `json:"last_sync,omitempty"`
}

func NewContactSyncService(contactSvc *ContactService, contactRepo *repository.ContactRepository, syncHistoryRepo *repository.SyncHistoryRepository) *ContactSyncService {
	return &ContactSyncService{
		contactSvc:      contactSvc,
		contactRepo:     contactRepo,
		syncHistoryRepo: syncHistoryRepo,
		status:          &ContactSyncStatus{},
	}
}

func (s *ContactSyncService) SyncContactsFull() {
	if !s.TryStartRunning() {
		slog.Info("contact sync already running, skipping")
		return
	}
	startTime := time.Now()

	finish := func(phase string, errMsg string) {
		s.mu.Lock()
		s.status.Running = false
		if errMsg != "" {
			s.status.Phase = "error"
			s.status.ErrorMsg = errMsg
		} else {
			s.status.Phase = "done"
		}
		s.status.LastSync = time.Now()
		s.mu.Unlock()
	}

	// 1. 拉取部门
	depts, err := s.contactSvc.GetDepartments()
	if err != nil {
		slog.Info(fmt.Sprintf("contact sync: get departments failed: %v", err))
		finish("departments", fmt.Sprintf("拉取部门失败: %v", err))
		return
	}
	var deptModels []model.Department
	for _, d := range depts {
		deptModels = append(deptModels, repository.DeptItemToDepartment(d))
	}
	if err := s.contactRepo.BatchUpsertDepts(deptModels); err != nil {
		slog.Error(fmt.Sprintf("contact sync: upsert departments failed: %v (sync continues)", err))
	}

	s.mu.Lock()
	s.status.Phase = "members"
	s.mu.Unlock()

	// 2. 按部门逐个拉取直接成员，避免根部门递归接口一次承载全量成员。
	simpleUsers, lastErr := s.fetchSimpleUsersByDepartment(depts, "contact sync")
	if len(simpleUsers) == 0 && lastErr != nil {
		slog.Info("contact sync: all department member fetches failed")
		finish("members", fmt.Sprintf("拉取成员列表失败: %v", lastErr))
		return
	}

	allUserIDs := simpleUserIDs(simpleUsers)

	s.mu.Lock()
	s.status.Total = len(allUserIDs)
	s.mu.Unlock()

	// 3. 比对本地已有用户
	existing, err := s.contactRepo.GetAllUserIDs()
	if err != nil {
		slog.Info(fmt.Sprintf("contact sync: get existing user ids failed: %v", err))
		finish("details", fmt.Sprintf("查询本地用户失败: %v", err))
		return
	}
	newUserIDs := repository.FilterNewUserIDs(allUserIDs, existing)
	missingUserIDs := repository.FilterMissingUserIDs(allUserIDs, existing)

	s.mu.Lock()
	s.status.Phase = "details"
	s.status.Total = len(newUserIDs)
	s.status.Progress = 0
	s.mu.Unlock()

	// 4. 新用户拉取详情
	if len(newUserIDs) > 0 {
		s.fetchAndSaveNewContacts("contact sync", newUserIDs)
	}

	// 5. 已有用户只更新基础字段，不覆盖 mobile/email/avatar 等详情
	var simpleContacts []model.Contact
	for _, u := range simpleUsers {
		if existing[u.UserID] {
			simpleContacts = append(simpleContacts, repository.SimpleUserToContact(u))
		}
	}
	if len(simpleContacts) > 0 {
		if err := s.contactRepo.BatchUpdateBasicInfo(simpleContacts); err != nil {
			slog.Info(fmt.Sprintf("contact sync: batch update basic info failed: %v", err))
		} else {
			s.mu.Lock()
			s.status.UpdatedCount = len(simpleContacts)
			s.mu.Unlock()
		}
	}

	if len(missingUserIDs) > 0 {
		slog.Info(fmt.Sprintf("contact sync: %d users no longer in API", len(missingUserIDs)))
		if err := s.contactRepo.MarkContactsInactive(missingUserIDs); err != nil {
			slog.Info(fmt.Sprintf("contact sync: mark inactive failed: %v", err))
		} else {
			s.mu.Lock()
			s.status.DeletedCount = len(missingUserIDs)
			s.mu.Unlock()
		}
	}

	slog.Info(fmt.Sprintf("contact sync: completed. new=%d, updated=%d, inactive=%d, failed=%d",
		s.status.NewCount, s.status.UpdatedCount, s.status.DeletedCount, s.status.FailedCount))
	s.saveContactSyncHistory("full", startTime, "")
	finish("", "")
}

func (s *ContactSyncService) SyncContactsIncremental() {
	if !s.TryStartRunning() {
		slog.Info("contact sync already running, skipping")
		return
	}
	startTime := time.Now()

	finish := func(errMsg string) {
		s.mu.Lock()
		s.status.Running = false
		if errMsg != "" {
			s.status.Phase = "error"
			s.status.ErrorMsg = errMsg
		} else {
			s.status.Phase = "done"
		}
		s.status.LastSync = time.Now()
		s.mu.Unlock()
	}

	// 1. 拉取部门
	depts, err := s.contactSvc.GetDepartments()
	if err != nil {
		slog.Info(fmt.Sprintf("contact incremental sync: get departments failed: %v", err))
		finish(fmt.Sprintf("拉取部门失败: %v", err))
		return
	}
	var deptModels []model.Department
	for _, d := range depts {
		deptModels = append(deptModels, repository.DeptItemToDepartment(d))
	}
	if err := s.contactRepo.BatchUpsertDepts(deptModels); err != nil {
		slog.Error(fmt.Sprintf("contact incremental sync: upsert departments failed: %v (sync continues)", err))
	}

	s.mu.Lock()
	s.status.Phase = "members"
	s.mu.Unlock()

	// 2. 按部门逐个拉取直接成员，避免根部门递归接口一次承载全量成员。
	simpleUsers, lastErr := s.fetchSimpleUsersByDepartment(depts, "contact incremental sync")
	if len(simpleUsers) == 0 && lastErr != nil {
		slog.Info("contact incremental sync: all department member fetches failed")
		finish(fmt.Sprintf("拉取成员列表失败: %v", lastErr))
		return
	}

	allUserIDs := simpleUserIDs(simpleUsers)

	s.mu.Lock()
	s.status.Total = len(allUserIDs)
	s.mu.Unlock()

	// 3. 比对本地
	existing, err := s.contactRepo.GetAllUserIDs()
	if err != nil {
		slog.Info(fmt.Sprintf("contact incremental sync: get existing user ids failed: %v", err))
		finish(fmt.Sprintf("查询本地用户失败: %v", err))
		return
	}
	newUserIDs := repository.FilterNewUserIDs(allUserIDs, existing)
	missingUserIDs := repository.FilterMissingUserIDs(allUserIDs, existing)

	s.mu.Lock()
	s.status.Phase = "details"
	s.status.Total = len(newUserIDs)
	s.status.Progress = 0
	s.mu.Unlock()

	// 4. 新用户拉取详情
	if len(newUserIDs) > 0 {
		s.fetchAndSaveNewContacts("contact incremental sync", newUserIDs)
	}

	// 5. 已有用户刷新基础字段和部门关系，避免组织架构变更滞后。
	var existingContacts []model.Contact
	for _, u := range simpleUsers {
		if existing[u.UserID] {
			existingContacts = append(existingContacts, repository.SimpleUserToContact(u))
		}
	}
	if len(existingContacts) > 0 {
		if err := s.contactRepo.BatchUpdateBasicInfo(existingContacts); err != nil {
			slog.Info(fmt.Sprintf("contact incremental sync: batch update basic info failed: %v", err))
		} else {
			s.mu.Lock()
			s.status.UpdatedCount = len(existingContacts)
			s.mu.Unlock()
		}
	}

	// 6. 标记不在 API 中的用户为失效。
	if len(missingUserIDs) > 0 {
		slog.Info(fmt.Sprintf("contact incremental sync: %d users no longer in API", len(missingUserIDs)))
		if err := s.contactRepo.MarkContactsInactive(missingUserIDs); err != nil {
			slog.Info(fmt.Sprintf("contact incremental sync: mark inactive failed: %v", err))
		} else {
			s.mu.Lock()
			s.status.DeletedCount = len(missingUserIDs)
			s.mu.Unlock()
		}
	}

	slog.Info(fmt.Sprintf("contact incremental sync: completed. new=%d, updated=%d, inactive=%d, failed=%d",
		s.status.NewCount, s.status.UpdatedCount, s.status.DeletedCount, s.status.FailedCount))
	s.saveContactSyncHistory("incremental", startTime, "")
	finish("")
}

func (s *ContactSyncService) fetchSimpleUsersByDepartment(depts []model.DepartmentItem, logPrefix string) ([]model.SimpleUser, error) {
	usersByID := make(map[string]model.SimpleUser)
	var order []string
	var lastErr error

	for _, d := range depts {
		select {
		case <-s.cancelCh:
			slog.Info(fmt.Sprintf("%s: cancelled while fetching department members", logPrefix))
			return mergeSimpleUsersInOrder(usersByID, order), nil
		default:
		}

		users, err := s.contactSvc.GetSimpleUserList(d.ID, 0)
		if err != nil {
			slog.Info(fmt.Sprintf("%s: department %d (%s) failed: %v", logPrefix, d.ID, d.Name, err))
			lastErr = err
			continue
		}
		slog.Info(fmt.Sprintf("%s: got %d direct users from department %d (%s)", logPrefix, len(users), d.ID, d.Name))
		for _, user := range users {
			if user.UserID == "" {
				continue
			}
			if _, ok := usersByID[user.UserID]; !ok {
				order = append(order, user.UserID)
			}
			usersByID[user.UserID] = mergeSimpleUser(usersByID[user.UserID], user)
		}
	}

	return mergeSimpleUsersInOrder(usersByID, order), lastErr
}

func (s *ContactSyncService) fetchAndSaveNewContacts(logPrefix string, userIDs []string) {
	batches := chunkStringSlice(userIDs, contactDetailBatchSize)
	for i, batch := range batches {
		select {
		case <-s.cancelCh:
			slog.Info(fmt.Sprintf("%s: cancelled while fetching details", logPrefix))
			return
		default:
		}

		contacts, failed := s.contactSvc.FetchAllDetails(batch, contactDetailConcurrency, s.cancelCh)
		if len(failed) > 0 {
			slog.Info(fmt.Sprintf("%s: retrying %d failed detail requests in batch %d/%d", logPrefix, len(failed), i+1, len(batches)))
			retryContacts, retryFailed := s.contactSvc.FetchAllDetails(failed, contactDetailConcurrency, s.cancelCh)
			contacts = append(contacts, retryContacts...)
			failed = retryFailed
		}

		if err := s.contactRepo.BatchUpsertContacts(contacts); err != nil {
			slog.Info(fmt.Sprintf("%s: batch upsert contacts failed: %v", logPrefix, err))
			failed = append(failed, contactUserIDs(contacts)...)
			contacts = nil
		}

		s.mu.Lock()
		s.status.NewCount += len(contacts)
		s.status.FailedCount += len(failed)
		s.status.Progress += len(contacts) + len(failed)
		s.mu.Unlock()

		slog.Info(fmt.Sprintf("%s: detail batch %d/%d done (ok=%d, failed=%d)",
			logPrefix, i+1, len(batches), len(contacts), len(failed)))
	}
}

func mergeSimpleUser(existing model.SimpleUser, incoming model.SimpleUser) model.SimpleUser {
	if existing.UserID == "" {
		return incoming
	}
	if existing.Name == "" {
		existing.Name = incoming.Name
	}
	existing.Department = mergeInts(existing.Department, incoming.Department)
	existing.IsLeaderInDept = mergeInts(existing.IsLeaderInDept, incoming.IsLeaderInDept)
	existing.Positions = mergeStrings(existing.Positions, incoming.Positions)
	return existing
}

func mergeSimpleUsersInOrder(usersByID map[string]model.SimpleUser, order []string) []model.SimpleUser {
	users := make([]model.SimpleUser, 0, len(order))
	for _, id := range order {
		users = append(users, usersByID[id])
	}
	return users
}

func simpleUserIDs(users []model.SimpleUser) []string {
	ids := make([]string, 0, len(users))
	for _, user := range users {
		if user.UserID != "" {
			ids = append(ids, user.UserID)
		}
	}
	return ids
}

func chunkStringSlice(values []string, size int) [][]string {
	if len(values) == 0 {
		return nil
	}
	if size <= 0 {
		size = len(values)
	}
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

func mergeInts(a []int, b []int) []int {
	seen := make(map[int]bool, len(a)+len(b))
	result := make([]int, 0, len(a)+len(b))
	for _, v := range append(a, b...) {
		if seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
	}
	return result
}

func mergeStrings(a []string, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))
	for _, v := range append(a, b...) {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
	}
	return result
}

func contactUserIDs(contacts []model.Contact) []string {
	userIDs := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		if contact.UserID != "" {
			userIDs = append(userIDs, contact.UserID)
		}
	}
	return userIDs
}

func (s *ContactSyncService) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelCh != nil {
		close(s.cancelCh)
		s.cancelCh = nil
	}
}

func (s *ContactSyncService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status.Running
}

// TryStartRunning 原子性地检查并设置运行状态
func (s *ContactSyncService) TryStartRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.Running {
		return false
	}
	s.status.Running = true
	s.status.Phase = "departments"
	s.status.Progress = 0
	s.status.Total = 0
	s.status.NewCount = 0
	s.status.UpdatedCount = 0
	s.status.DeletedCount = 0
	s.status.FailedCount = 0
	s.status.ErrorMsg = ""
	s.cancelCh = make(chan struct{})
	return true
}

func (s *ContactSyncService) ResetRunning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Running = false
}

// StartSync 在后台 goroutine 中启动同步，自动处理 TryStartRunning、panic 恢复和 ResetRunning。
func (s *ContactSyncService) StartSync(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error(fmt.Sprintf("contact sync goroutine panic: %v\n%s", r, debug.Stack()))
				s.ResetRunning()
			}
		}()
		fn()
	}()
}

func (s *ContactSyncService) GetStatus() *ContactSyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *s.status
	return &copy
}

func (s *ContactSyncService) saveContactSyncHistory(trigger string, startTime time.Time, errMsg string) {
	s.mu.Lock()
	newCount := s.status.NewCount
	updatedCount := s.status.UpdatedCount
	deletedCount := s.status.DeletedCount
	failedCount := s.status.FailedCount
	total := s.status.Total
	s.mu.Unlock()

	detailsMap := map[string]interface{}{
		"new_count":     newCount,
		"updated_count": updatedCount,
		"deleted_count": deletedCount,
		"failed_count":  failedCount,
		"total_users":   total,
	}
	detailsJSON, _ := json.Marshal(detailsMap)

	history := &model.SyncHistory{
		SyncType:   "contact",
		Trigger:    trigger,
		StartTime:  startTime,
		EndTime:    time.Now(),
		DurationMs: time.Since(startTime).Milliseconds(),
		Total:      total,
		Succeeded:  newCount,
		Failed:     failedCount,
		Details:    string(detailsJSON),
		ErrorMsg:   errMsg,
	}
	if err := s.syncHistoryRepo.Create(history); err != nil {
		slog.Info(fmt.Sprintf("save contact sync history failed: %v", err))
	}
}

// SyncContactsTask 处理来自队列的任务
func (s *ContactSyncService) SyncContactsTask(task *model.SyncTask) (map[string]interface{}, error) {
	// 判断是增量还是全量同步
	syncType := "incremental"
	if task.EndTime > 0 {
		syncType = "full"
	}

	if syncType == "full" {
		s.SyncContactsFull()
	} else {
		s.SyncContactsIncremental()
	}

	return map[string]interface{}{
		"sync_type": syncType,
	}, nil
}
