package service

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type ContactAsyncSyncService struct {
	asyncExportSvc *AsyncExportService
	mediaSvc       *MediaService
	csvExportSvc   *CSVExportService
	contactRepo    *repository.ContactRepository
	syncHistoryRepo *repository.SyncHistoryRepository
	contactSvc     *ContactService
	status         *AsyncSyncStatus
	mu             sync.Mutex
}

type AsyncSyncStatus struct {
	Running      bool      `json:"running"`
	Phase        string    `json:"phase"`
	Progress     int       `json:"progress"`
	Total        int       `json:"total"`
	Imported     int       `json:"imported"`
	Failed       int       `json:"failed"`
	JobID        string    `json:"job_id,omitempty"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
	LastSync     time.Time `json:"last_sync,omitempty"`
	SyncType     string    `json:"sync_type,omitempty"`
}

func NewContactAsyncSyncService(
	asyncExportSvc *AsyncExportService,
	mediaSvc *MediaService,
	csvExportSvc *CSVExportService,
	contactRepo *repository.ContactRepository,
	syncHistoryRepo *repository.SyncHistoryRepository,
	contactSvc *ContactService,
) *ContactAsyncSyncService {
	return &ContactAsyncSyncService{
		asyncExportSvc:  asyncExportSvc,
		mediaSvc:        mediaSvc,
		csvExportSvc:    csvExportSvc,
		contactRepo:     contactRepo,
		syncHistoryRepo: syncHistoryRepo,
		contactSvc:      contactSvc,
		status:          &AsyncSyncStatus{},
	}
}

func (s *ContactAsyncSyncService) TryStartRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.Running {
		return false
	}
	s.status.Running = true
	s.status.Phase = "init"
	s.status.Progress = 0
	s.status.Total = 0
	s.status.Imported = 0
	s.status.Failed = 0
	s.status.ErrorMsg = ""
	s.status.JobID = ""
	return true
}

func (s *ContactAsyncSyncService) GetStatus() *AsyncSyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *s.status
	return &copy
}

func (s *ContactAsyncSyncService) ResetRunning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Running = false
}

type SyncResult struct {
	Total     int
	Imported  int
	Failed    int
	JobID     string
	Duration  time.Duration
}

func (s *ContactAsyncSyncService) SyncAllAsync(departmentID int, fetchChild int) (*SyncResult, error) {
	startTime := time.Now()

	if !s.TryStartRunning() {
		return nil, fmt.Errorf("sync is already running")
	}
	defer s.ResetRunning()

	s.mu.Lock()
	s.status.SyncType = "async_export"
	s.mu.Unlock()

	jobID, err := s.asyncExportSvc.StartExport(departmentID, fetchChild)
	if err != nil {
		s.setError(fmt.Sprintf("start export failed: %v", err))
		return nil, err
	}

	s.mu.Lock()
	s.status.JobID = jobID
	s.status.Phase = "exporting"
	s.mu.Unlock()

	log.Printf("ContactAsyncSync: exported job %s started", jobID)

	result, err := s.asyncExportSvc.PollExportResult(jobID, 30*time.Minute, 2*time.Second)
	if err != nil {
		s.setError(fmt.Sprintf("poll export result failed: %v", err))
		return nil, err
	}

	s.mu.Lock()
	s.status.Phase = "importing"
	s.status.Total = len(result.Users)
	s.mu.Unlock()

	log.Printf("ContactAsyncSync: export completed, got %d users, processing...", len(result.Users))

	contacts := make([]interface{}, 0, len(result.Users))
	for _, u := range result.Users {
		contacts = append(contacts, ExportedUserToContact(u))
	}

	imported, failed, err := s.contactRepo.BatchUpsertContactsFromExport(contacts)
	if err != nil {
		log.Printf("ContactAsyncSync: batch upsert failed: %v", err)
	}

	s.mu.Lock()
	s.status.Imported = imported
	s.status.Failed = failed
	s.status.Phase = "done"
	s.mu.Unlock()

	duration := time.Since(startTime)
	log.Printf("ContactAsyncSync: completed. imported=%d, failed=%d, duration=%v", imported, failed, duration)

	s.saveSyncHistory("async_export", startTime, "", imported, failed, len(result.Users))

	return &SyncResult{
		Total:    len(result.Users),
		Imported: imported,
		Failed:   failed,
		JobID:    jobID,
		Duration: duration,
	}, nil
}

func (s *ContactAsyncSyncService) SyncIncrementalAsync() (*SyncResult, error) {
	startTime := time.Now()

	if !s.TryStartRunning() {
		return nil, fmt.Errorf("sync is already running")
	}
	defer s.ResetRunning()

	s.mu.Lock()
	s.status.SyncType = "incremental_async"
	s.mu.Unlock()

	s.mu.Lock()
	s.status.Phase = "comparing"
	s.mu.Unlock()

	depts, err := s.contactSvc.GetDepartments()
	if err != nil {
		s.setError(fmt.Sprintf("get departments failed: %v", err))
		return nil, err
	}

	var apiUsers []string
	userSeen := make(map[string]bool)
	for _, d := range depts {
		if d.ParentID == 0 {
			users, err := s.contactSvc.GetSimpleUserList(d.ID, 1)
			if err != nil {
				log.Printf("ContactAsyncSync: get users from dept %d failed: %v", d.ID, err)
				continue
			}
			for _, u := range users {
				if !userSeen[u.UserID] {
					userSeen[u.UserID] = true
					apiUsers = append(apiUsers, u.UserID)
				}
			}
		}
	}

	existingIDs, err := s.contactRepo.GetAllUserIDs()
	if err != nil {
		s.setError(fmt.Sprintf("get existing users failed: %v", err))
		return nil, err
	}

	var newUserIDs []string
	var missingUserIDs []string
	for _, uid := range apiUsers {
		if existingIDs[uid] {
			continue
		}
		newUserIDs = append(newUserIDs, uid)
	}
	for uid := range existingIDs {
		if !userSeen[uid] {
			missingUserIDs = append(missingUserIDs, uid)
		}
	}

	log.Printf("ContactAsyncSync: new=%d, missing=%d", len(newUserIDs), len(missingUserIDs))

	s.mu.Lock()
	s.status.Total = len(newUserIDs)
	s.status.Phase = "importing"
	s.mu.Unlock()

	if len(newUserIDs) > 0 {
		contacts, failedIDs := s.contactSvc.FetchAllDetails(newUserIDs, 5, nil)
		if err := s.contactRepo.BatchUpsertContacts(contacts); err != nil {
			log.Printf("ContactAsyncSync: batch upsert failed: %v", err)
		}

		s.mu.Lock()
		s.status.Imported = len(contacts)
		s.status.Failed = len(failedIDs)
		s.mu.Unlock()
	}

	duration := time.Since(startTime)
	log.Printf("ContactAsyncSync: incremental completed. imported=%d, duration=%v", s.status.Imported, duration)

	s.saveSyncHistory("incremental_async", startTime, "", s.status.Imported, s.status.Failed, len(apiUsers))

	s.mu.Lock()
	s.status.Phase = "done"
	s.mu.Unlock()

	return &SyncResult{
		Total:    len(newUserIDs),
		Imported: s.status.Imported,
		Failed:   s.status.Failed,
		Duration: duration,
	}, nil
}

func (s *ContactAsyncSyncService) setError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Phase = "error"
	s.status.ErrorMsg = msg
}

func (s *ContactAsyncSyncService) saveSyncHistory(trigger string, startTime time.Time, errMsg string, imported, failed, total int) {
	detailsMap := map[string]interface{}{
		"imported": imported,
		"failed":   failed,
		"total":    total,
	}
	detailsJSON, _ := json.Marshal(detailsMap)

	history := &model.SyncHistory{
		SyncType:   "contact_async",
		Trigger:    trigger,
		StartTime:  startTime,
		EndTime:    time.Now(),
		DurationMs: time.Since(startTime).Milliseconds(),
		Total:      total,
		Succeeded:  imported,
		Failed:     failed,
		Details:    string(detailsJSON),
		ErrorMsg:   errMsg,
	}
	if err := s.syncHistoryRepo.Create(history); err != nil {
		log.Printf("save contact async sync history failed: %v", err)
	}
}
