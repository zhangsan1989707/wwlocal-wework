package service

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type ContactSchedulerService struct {
	contactSyncSvc *ContactSyncService
	interval       time.Duration
	startDelay     time.Duration
	ticker         *time.Ticker
	stopCh         chan struct{}
	running        bool
	mu             sync.Mutex
	nextRun        time.Time
	lastRun        time.Time
	wg             sync.WaitGroup
}

func NewContactSchedulerService(contactSyncSvc *ContactSyncService, interval, startDelay time.Duration) *ContactSchedulerService {
	return &ContactSchedulerService{
		contactSyncSvc: contactSyncSvc,
		interval:       interval,
		startDelay:     startDelay,
	}
}

func (s *ContactSchedulerService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}

	s.ticker = time.NewTicker(s.interval)
	s.stopCh = make(chan struct{})
	s.running = true
	s.nextRun = time.Now().Add(s.startDelay)

	s.wg.Add(1)
	go s.run()
	slog.Info(fmt.Sprintf("contact scheduler started, interval: %v, first run in: %v", s.interval, s.startDelay))
}

func (s *ContactSchedulerService) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	close(s.stopCh)
	s.ticker.Stop()
	s.running = false
	slog.Info("contact scheduler stopping...")
	s.mu.Unlock()

	s.wg.Wait()
	slog.Info("contact scheduler stopped")
}

func (s *ContactSchedulerService) run() {
	defer s.wg.Done()

	select {
	case <-time.After(s.startDelay):
		s.doSync()
	case <-s.stopCh:
		return
	}

	for {
		select {
		case <-s.ticker.C:
			s.doSync()
		case <-s.stopCh:
			return
		}
	}
}

func (s *ContactSchedulerService) doSync() {
	if s.contactSyncSvc.IsRunning() {
		slog.Info("contact scheduler: contact sync already running, skipping this tick")
		return
	}

	s.mu.Lock()
	s.lastRun = time.Now()
	s.nextRun = s.lastRun.Add(s.interval)
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error(fmt.Sprintf("contact scheduler: sync goroutine panic: %v", r))
				s.contactSyncSvc.ResetRunning()
			}
		}()
		slog.Info("contact scheduler: starting incremental contact sync")
		s.contactSyncSvc.SyncContactsIncremental()
	}()
}

func (s *ContactSchedulerService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
