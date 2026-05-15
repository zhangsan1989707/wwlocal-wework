package service

import (
	"log"
	"sync"
	"time"
)

type SchedulerService struct {
	syncSvc   *SyncService
	interval  time.Duration
	ticker    *time.Ticker
	stopCh    chan struct{}
	running   bool
	mu        sync.Mutex
	nextRun   time.Time
	lastRun   time.Time
}

func NewSchedulerService(syncSvc *SyncService, interval time.Duration) *SchedulerService {
	return &SchedulerService{
		syncSvc:  syncSvc,
		interval: interval,
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
	log.Printf("scheduler started, interval: %v, first run in: %v", s.interval, startDelay)
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
	log.Printf("scheduler stopped")
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
		log.Printf("scheduler: sync already running, skipping this tick")
		return
	}
	log.Printf("scheduler: starting incremental sync")
	s.mu.Lock()
	s.lastRun = time.Now()
	s.mu.Unlock()
	go func() {
		if !s.syncSvc.TryStartRunning() {
			log.Printf("scheduler: sync already started by another caller, skipping")
			return
		}
		results := s.syncSvc.SyncAllFeaturesIncremental()
		total := 0
		for _, count := range results {
			if count > 0 {
				total += count
			}
		}
		log.Printf("scheduler: incremental sync completed, total fetched: %d", total)
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
