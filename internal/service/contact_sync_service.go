package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type ContactSyncService struct {
	contactSvc  *ContactService
	contactRepo *repository.ContactRepository
	mu          sync.Mutex
	status      *ContactSyncStatus
	cancelCh    chan struct{}
}

type ContactSyncStatus struct {
	Running     bool      `json:"running"`
	Phase       string    `json:"phase"`
	Progress    int       `json:"progress"`
	Total       int       `json:"total"`
	NewCount    int       `json:"new_count"`
	FailedCount int       `json:"failed_count"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
	LastSync    time.Time `json:"last_sync,omitempty"`
}

func NewContactSyncService(contactSvc *ContactService, contactRepo *repository.ContactRepository) *ContactSyncService {
	return &ContactSyncService{
		contactSvc:  contactSvc,
		contactRepo: contactRepo,
		status:      &ContactSyncStatus{},
	}
}

func (s *ContactSyncService) SyncContactsFull() {
	s.mu.Lock()
	s.status.Running = true
	s.status.Phase = "departments"
	s.status.Progress = 0
	s.status.Total = 0
	s.status.NewCount = 0
	s.status.FailedCount = 0
	s.status.ErrorMsg = ""
	s.cancelCh = make(chan struct{})
	s.mu.Unlock()

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
		log.Printf("contact sync: get departments failed: %v", err)
		finish("departments", fmt.Sprintf("拉取部门失败: %v", err))
		return
	}
	var deptModels []model.Department
	for _, d := range depts {
		deptModels = append(deptModels, repository.DeptItemToDepartment(d))
	}
	if err := s.contactRepo.BatchUpsertDepts(deptModels); err != nil {
		log.Printf("contact sync: upsert departments failed: %v", err)
	}

	s.mu.Lock()
	s.status.Phase = "members"
	s.mu.Unlock()

	// 2. 拉取所有成员简略列表 — 尝试各根部门直到成功
	var simpleUsers []model.SimpleUser
	var simpleErr error
	for _, d := range depts {
		if d.ParentID == 0 {
			simpleUsers, simpleErr = s.contactSvc.GetSimpleUserList(d.ID, 1)
			if simpleErr == nil {
				log.Printf("contact sync: got %d users from department %d (%s)", len(simpleUsers), d.ID, d.Name)
				break
			}
			log.Printf("contact sync: department %d (%s) failed: %v", d.ID, d.Name, simpleErr)
		}
	}
	if simpleErr != nil {
		log.Printf("contact sync: all root departments failed")
		finish("members", fmt.Sprintf("拉取成员列表失败: %v", simpleErr))
		return
	}

	allUserIDs := make([]string, len(simpleUsers))
	for i, u := range simpleUsers {
		allUserIDs[i] = u.UserID
	}

	s.mu.Lock()
	s.status.Total = len(allUserIDs)
	s.mu.Unlock()

	// 3. 比对本地已有用户
	existing, err := s.contactRepo.GetAllUserIDs()
	if err != nil {
		log.Printf("contact sync: get existing user ids failed: %v", err)
		finish("details", fmt.Sprintf("查询本地用户失败: %v", err))
		return
	}
	newUserIDs := repository.FilterNewUserIDs(allUserIDs, existing)

	s.mu.Lock()
	s.status.Phase = "details"
	s.status.Total = len(newUserIDs)
	s.status.Progress = 0
	s.mu.Unlock()

	// 4. 新用户拉取详情
	if len(newUserIDs) > 0 {
		contacts, failed := s.contactSvc.FetchAllDetails(newUserIDs, 5, s.cancelCh)
		if err := s.contactRepo.BatchUpsertContacts(contacts); err != nil {
			log.Printf("contact sync: batch upsert contacts failed: %v", err)
		}
		s.mu.Lock()
		s.status.NewCount = len(contacts)
		s.status.FailedCount = len(failed)
		s.status.Progress = len(contacts) + len(failed)
		s.mu.Unlock()
	}

	// 5. 已有用户只更新基础字段，不覆盖 mobile/email/avatar 等详情
	var simpleContacts []model.Contact
	for _, u := range simpleUsers {
		if existing[u.UserID] {
			simpleContacts = append(simpleContacts, repository.SimpleUserToContact(u))
		}
	}
	if len(simpleContacts) > 0 {
		s.contactRepo.BatchUpdateBasicInfo(simpleContacts)
	}

	log.Printf("contact sync: completed. new=%d, failed=%d", s.status.NewCount, s.status.FailedCount)
	finish("", "")
}

func (s *ContactSyncService) SyncContactsIncremental() {
	s.mu.Lock()
	s.status.Running = true
	s.status.Phase = "departments"
	s.status.Progress = 0
	s.status.Total = 0
	s.status.NewCount = 0
	s.status.FailedCount = 0
	s.status.ErrorMsg = ""
	s.cancelCh = make(chan struct{})
	s.mu.Unlock()

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
		log.Printf("contact incremental sync: get departments failed: %v", err)
		finish(fmt.Sprintf("拉取部门失败: %v", err))
		return
	}
	var deptModels []model.Department
	for _, d := range depts {
		deptModels = append(deptModels, repository.DeptItemToDepartment(d))
	}
	s.contactRepo.BatchUpsertDepts(deptModels)

	s.mu.Lock()
	s.status.Phase = "members"
	s.mu.Unlock()

	// 2. 拉取所有成员简略列表 — 尝试各根部门直到成功
	var simpleUsers []model.SimpleUser
	var simpleErr error
	for _, d := range depts {
		if d.ParentID == 0 {
			simpleUsers, simpleErr = s.contactSvc.GetSimpleUserList(d.ID, 1)
			if simpleErr == nil {
				log.Printf("contact incremental sync: got %d users from department %d (%s)", len(simpleUsers), d.ID, d.Name)
				break
			}
			log.Printf("contact incremental sync: department %d (%s) failed: %v", d.ID, d.Name, simpleErr)
		}
	}
	if simpleErr != nil {
		log.Printf("contact incremental sync: all root departments failed")
		finish(fmt.Sprintf("拉取成员列表失败: %v", simpleErr))
		return
	}

	allUserIDs := make([]string, len(simpleUsers))
	for i, u := range simpleUsers {
		allUserIDs[i] = u.UserID
	}

	s.mu.Lock()
	s.status.Total = len(allUserIDs)
	s.mu.Unlock()

	// 3. 比对本地
	existing, err := s.contactRepo.GetAllUserIDs()
	if err != nil {
		log.Printf("contact incremental sync: get existing user ids failed: %v", err)
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

	// 4. 只对新用户拉取详情
	if len(newUserIDs) > 0 {
		contacts, failed := s.contactSvc.FetchAllDetails(newUserIDs, 5, s.cancelCh)
		if err := s.contactRepo.BatchUpsertContacts(contacts); err != nil {
			log.Printf("contact incremental sync: batch upsert failed: %v", err)
		}
		s.mu.Lock()
		s.status.NewCount = len(contacts)
		s.status.FailedCount = len(failed)
		s.status.Progress = len(contacts) + len(failed)
		s.mu.Unlock()
	}

	// 5. 已有用户更新 synced_at
	var existingIDs []string
	for _, id := range allUserIDs {
		if existing[id] {
			existingIDs = append(existingIDs, id)
		}
	}
	if len(existingIDs) > 0 {
		s.contactRepo.MarkSyncedAt(existingIDs)
	}

	// 6. 标记不在 API 中的用户
	if len(missingUserIDs) > 0 {
		log.Printf("contact incremental sync: %d users no longer in API", len(missingUserIDs))
	}

	log.Printf("contact incremental sync: completed. new=%d, failed=%d", s.status.NewCount, s.status.FailedCount)
	finish("")
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

func (s *ContactSyncService) GetStatus() *ContactSyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *s.status
	return &copy
}
