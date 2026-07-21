package backup

import (
	"fmt"
	"sync"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"gorm.io/gorm"
)

// nowFunc returns the current time; created backups stamp CreatedAt with it.
type nowFunc func() time.Time

// Scheduler runs per-server automatic backups on a fixed interval. Each enabled
// schedule gets its own goroutine + ticker. It is started once at boot and
// reloaded whenever a server's schedule changes. Backup failures are logged and
// never crash the scheduler, matching the tolerant style of the update checker.
type Scheduler struct {
	db  *gorm.DB
	svc *Service
	now nowFunc

	mu      sync.Mutex
	tickers map[int64]*schedTicker // serverID -> running ticker
	started bool
}

type schedTicker struct {
	stop chan struct{}
}

// NewScheduler constructs a Scheduler over a backup Service.
func NewScheduler(db *gorm.DB, svc *Service) *Scheduler {
	return &Scheduler{
		db:      db,
		svc:     svc,
		now:     time.Now,
		tickers: make(map[int64]*schedTicker),
	}
}

// Start launches goroutines for every currently-enabled schedule. Safe to call
// once at boot. Subsequent config changes go through Reload.
func (s *Scheduler) Start() {
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()

	var scheds []models.BackupSchedule
	if err := s.db.Where("enabled = ?", true).Find(&scheds).Error; err != nil {
		fmt.Printf("warning: backup scheduler start: load schedules: %v\n", err)
		return
	}
	for _, sc := range scheds {
		s.startOne(sc)
	}
}

// Reload restarts the ticker for a single server to reflect its current
// schedule row. Called after the schedule is updated via the API. Stops any
// existing ticker first; starts a new one only if the schedule is enabled with
// a positive interval.
func (s *Scheduler) Reload(serverID int64) {
	var sc models.BackupSchedule
	err := s.db.First(&sc, "server_id = ?", serverID).Error
	if err != nil {
		// No row (or read error) → ensure no ticker is running.
		s.stopOne(serverID)
		return
	}
	s.stopOne(serverID)
	if sc.Enabled {
		s.startOne(sc)
	}
}

// startOne launches a ticker goroutine for one schedule. A non-positive
// interval is ignored (nothing to schedule).
func (s *Scheduler) startOne(sc models.BackupSchedule) {
	if !sc.Enabled || sc.IntervalMinutes <= 0 {
		return
	}
	interval := time.Duration(sc.IntervalMinutes) * time.Minute
	scope := sc.Scope
	if !validScope(scope) {
		scope = models.BackupScopeAll
	}
	serverID := sc.ServerID

	stop := make(chan struct{})
	s.mu.Lock()
	s.tickers[serverID] = &schedTicker{stop: stop}
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if _, err := s.svc.Create(serverID, scope, models.BackupSourceAuto, s.now()); err != nil {
					fmt.Printf("warning: auto-backup server %d: %v\n", serverID, err)
				}
			}
		}
	}()
}

// stopOne stops and forgets a server's ticker if present.
func (s *Scheduler) stopOne(serverID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tickers[serverID]; ok {
		close(t.stop)
		delete(s.tickers, serverID)
	}
}

// Stop halts every ticker. Used on shutdown / in tests.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.tickers {
		close(t.stop)
		delete(s.tickers, id)
	}
}
