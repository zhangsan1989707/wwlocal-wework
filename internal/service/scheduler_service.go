package service

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type SchedulerService struct {
	syncSvc      *SyncService
	adminLogSvc  *AdminOperLogService
	interval     time.Duration
	ticker       *time.Ticker
	stopCh       chan struct{}
	running      bool
	mu           sync.Mutex
	nextRun      time.Time
	lastRun      time.Time
}

func NewSchedulerService(syncSvc *SyncService, adminLogSvc *AdminOperLogService, interval time.Duration) *SchedulerService {
	return &SchedulerService{
		syncSvc:     syncSvc,
		adminLogSvc: adminLogSvc,
		interval:    interval,
	}
}

func (s *SchedulerService) Start(startDelay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}

	s.ticker = time.NewTicker(s.interval)
	s.stopCh = make(chan struct{})
	s.running = true
	s.nextRun = time.Now().Add(startDelay)

	go s.run(startDelay)
	slog.Info(fmt.Sprintf("scheduler started, interval: %v, first run in: %v", s.interval, startDelay))
}

func (s *SchedulerService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}

	close(s.stopCh)
	s.ticker.Stop()
	s.running = false
	s.syncSvc.Cancel()
	slog.Info(fmt.Sprintf("scheduler stopped"))
}

func (s *SchedulerService) run(startDelay time.Duration) {
	// 首次延迟执行
	select {
	case <-time.After(startDelay):
	case <-s.stopCh:
		return
	}
	s.doSync()

	// 后续按 interval 周期执行
	for {
		select {
		case <-s.ticker.C:
			s.doSync()
		case <-s.stopCh:
			return
		}
	}
}

func (s *SchedulerService) doSync() {
	if s.syncSvc.IsRunning() {
		slog.Info(fmt.Sprintf("scheduler: sync already running, skipping this tick"))
		return
	}
	slog.Info(fmt.Sprintf("scheduler: starting incremental sync"))
	s.mu.Lock()
	s.lastRun = time.Now()
	s.mu.Unlock()
	go func() {
		defer s.syncSvc.ResetRunning()

		if !s.syncSvc.TryStartRunning() {
			slog.Info(fmt.Sprintf("scheduler: sync already started by another caller, skipping"))
			return
		}
		results := s.syncSvc.SyncAllFeaturesIncremental()
		total := 0
		for _, count := range results {
			if count > 0 {
				total += count
			}
		}
		slog.Info(fmt.Sprintf("scheduler: feature incremental sync completed, total fetched: %d", total))

		// 同步企微操作日志（在 ResetRunning 之前，防止并发冲突）
		if s.adminLogSvc != nil {
			if n, err := s.adminLogSvc.SyncIncremental(); err != nil {
				slog.Error("scheduler: admin oper log sync failed", "error", err)
			} else if n > 0 {
				slog.Info("scheduler: admin oper log sync completed", "fetched", n)
			}
		}

		s.mu.Lock()
		s.nextRun = time.Now().Add(s.interval)
		s.mu.Unlock()
	}()
}

func (s *SchedulerService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

type SchedulerStatus struct {
	Running  bool      `json:"running"`
	Interval string    `json:"interval"`
	NextRun  time.Time `json:"next_run,omitempty"`
	LastRun  time.Time `json:"last_run,omitempty"`
}

func (s *SchedulerService) GetStatus() SchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SchedulerStatus{
		Running:  s.running,
		Interval: s.interval.String(),
		NextRun:  s.nextRun,
		LastRun:  s.lastRun,
	}
}

func (s *SchedulerService) SetInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = d
	if s.running {
		s.ticker.Stop()
		s.ticker = time.NewTicker(d)
		s.nextRun = time.Now().Add(d)
	}
}

func (s *SchedulerService) GetInterval() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.interval
}
