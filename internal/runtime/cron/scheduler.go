package cron

import (
	"fmt"
	"sync"
	"time"

	platformstore "github.com/sheepzhao/claude-code-go/internal/platform/store"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	checkIntervalMs     = 1000
	lockProbeIntervalMs = 5000
)

// SchedulerOptions configures the cron scheduler.
type SchedulerOptions struct {
	ProjectRoot  string
	SessionID    string
	OnFire       func(prompt string)
	OnFireTask   func(task platformstore.CronTask)
	OnMissed     func(tasks []platformstore.CronTask)
	JitterConfig *CronJitterConfig
}

// Scheduler is the cron scheduler runtime responsible for executing scheduled
// tasks on their cron schedule.
type Scheduler struct {
	opts        SchedulerOptions
	jitterCfg   CronJitterConfig
	mu          sync.Mutex
	stopped     bool
	isOwner     bool
	tasks       []platformstore.CronTask
	nextFireAt  map[string]int64
	missedAsked map[string]struct{}
	inFlight    map[string]struct{}
	checkTimer  *time.Ticker
	lockProbe   *time.Ticker
}

// NewScheduler creates a new cron scheduler with the given options.
func NewScheduler(opts SchedulerOptions) *Scheduler {
	cfg := DefaultCronJitterConfig
	if opts.JitterConfig != nil {
		cfg = *opts.JitterConfig
	}
	if opts.SessionID == "" {
		opts.SessionID = GetPIDString()
	}
	return &Scheduler{
		opts:        opts,
		jitterCfg:   cfg,
		nextFireAt:  make(map[string]int64),
		missedAsked: make(map[string]struct{}),
		inFlight:    make(map[string]struct{}),
	}
}

// Start begins the scheduler. It attempts to acquire the scheduler lock; only
// the owner runs the check loop. Non-owners probe the lock periodically.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if !s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = false
	s.mu.Unlock()

	owned, err := TryAcquireSchedulerLock(s.opts.ProjectRoot, s.opts.SessionID)
	if err != nil {
		logger.Warn(fmt.Sprintf("cron scheduler: failed to acquire lock: %v", err))
	}
	s.mu.Lock()
	s.isOwner = owned
	s.mu.Unlock()

	if s.isOwner {
		s.loadTasks(true)
	} else {
		s.lockProbe = time.NewTicker(lockProbeIntervalMs * time.Millisecond)
		go s.runLockProbe()
	}

	s.checkTimer = time.NewTicker(checkIntervalMs * time.Millisecond)
	go s.runCheckLoop()
}

// Stop tears down the scheduler and releases the lock if owned.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	if s.checkTimer != nil {
		s.checkTimer.Stop()
		s.checkTimer = nil
	}
	if s.lockProbe != nil {
		s.lockProbe.Stop()
		s.lockProbe = nil
	}

	s.mu.Lock()
	if s.isOwner {
		s.isOwner = false
		s.mu.Unlock()
		if err := ReleaseSchedulerLock(s.opts.ProjectRoot, s.opts.SessionID); err != nil {
			logger.Warn(fmt.Sprintf("cron scheduler: failed to release lock: %v", err))
		}
	} else {
		s.mu.Unlock()
	}
}

// GetNextFireTime returns the earliest scheduled fire time across all loaded
// tasks, or nil if nothing is scheduled.
func (s *Scheduler) GetNextFireTime() *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	var min int64 = -1
	for _, t := range s.nextFireAt {
		if t <= 0 {
			continue
		}
		if min == -1 || t < min {
			min = t
		}
	}
	if min == -1 {
		return nil
	}
	tm := time.UnixMilli(min)
	return &tm
}

// loadTasks reads tasks from disk and runs missed-task detection on initial
// load.
func (s *Scheduler) loadTasks(initial bool) {
	tasks, err := platformstore.ReadCronTasks(s.opts.ProjectRoot)
	if err != nil {
		logger.Warn(fmt.Sprintf("cron scheduler: failed to read tasks: %v", err))
		return
	}

	s.mu.Lock()
	s.tasks = tasks
	s.mu.Unlock()

	if !initial {
		return
	}

	now := time.Now().UnixMilli()
	var cronTasks []CronTask
	for _, t := range tasks {
		cronTasks = append(cronTasks, cronTaskFromPlatform(t))
	}
	missed := FindMissedTasks(cronTasks, now)

	var missedNonRecurring []platformstore.CronTask
	for _, m := range missed {
		if !m.Recurring {
			id := m.ID
			s.mu.Lock()
			if _, asked := s.missedAsked[id]; !asked {
				s.missedAsked[id] = struct{}{}
				s.nextFireAt[id] = -1
				missedNonRecurring = append(missedNonRecurring, platformTaskFromCron(m))
			}
			s.mu.Unlock()
		}
	}

	if len(missedNonRecurring) > 0 {
		if s.opts.OnMissed != nil {
			s.opts.OnMissed(missedNonRecurring)
		}
		var ids []string
		for _, t := range missedNonRecurring {
			ids = append(ids, t.ID)
		}
		if err := platformstore.RemoveCronTasks(s.opts.ProjectRoot, ids); err != nil {
			logger.Warn(fmt.Sprintf("cron scheduler: failed to remove missed tasks: %v", err))
		}
	}
}

// runCheckLoop is the main scheduler tick loop.
func (s *Scheduler) runCheckLoop() {
	for range s.checkTimer.C {
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return
		}
		owner := s.isOwner
		s.mu.Unlock()

		if !owner {
			continue
		}
		s.check()
	}
}

// runLockProbe periodically tries to acquire the lock when we don't own it.
func (s *Scheduler) runLockProbe() {
	for range s.lockProbe.C {
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		owned, err := TryAcquireSchedulerLock(s.opts.ProjectRoot, s.opts.SessionID)
		if err != nil {
			continue
		}
		if owned {
			s.mu.Lock()
			s.isOwner = true
			if s.lockProbe != nil {
				s.lockProbe.Stop()
				s.lockProbe = nil
			}
			s.mu.Unlock()
			s.loadTasks(true)
			return
		}
	}
}

// check performs one tick of the scheduler: reload tasks, compute next fire
// times, fire due tasks, reschedule recurring, delete one-shot/aged.
func (s *Scheduler) check() {
	tasks, err := platformstore.ReadCronTasks(s.opts.ProjectRoot)
	if err != nil {
		return
	}

	now := time.Now().UnixMilli()
	seen := make(map[string]struct{})
	var firedRecurring []string

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks = tasks

	for _, t := range tasks {
		if _, inflight := s.inFlight[t.ID]; inflight {
			seen[t.ID] = struct{}{}
			continue
		}
		seen[t.ID] = struct{}{}

		ct := cronTaskFromPlatform(t)
		next := s.nextFireAt[t.ID]

		if next == 0 {
			anchor := ct.CreatedAt.UnixMilli()
			if ct.LastFiredAt != nil {
				anchor = ct.LastFiredAt.UnixMilli()
			}
			if ct.Recurring {
				next = JitteredNextCronRunMs(ct.Cron, anchor, ct.ID, s.jitterCfg)
			} else {
				next = OneShotJitteredNextCronRunMs(ct.Cron, anchor, ct.ID, s.jitterCfg)
			}
			if next == 0 {
				next = -1
			}
			s.nextFireAt[t.ID] = next
		}

		if next == -1 || now < next {
			continue
		}

		aged := IsRecurringTaskAged(ct, now, s.jitterCfg.RecurringMaxAgeMs)

		if s.opts.OnFireTask != nil {
			s.opts.OnFireTask(t)
		} else if s.opts.OnFire != nil {
			s.opts.OnFire(t.Prompt)
		}

		if ct.Recurring && !aged {
			newNext := JitteredNextCronRunMs(ct.Cron, now, ct.ID, s.jitterCfg)
			if newNext == 0 {
				newNext = -1
			}
			s.nextFireAt[t.ID] = newNext
			firedRecurring = append(firedRecurring, t.ID)
		} else {
			s.inFlight[t.ID] = struct{}{}
			delete(s.nextFireAt, t.ID)
			go func(taskID string) {
				if err := platformstore.RemoveCronTasks(s.opts.ProjectRoot, []string{taskID}); err != nil {
					logger.Warn(fmt.Sprintf("cron scheduler: failed to remove task %s: %v", taskID, err))
				}
				s.mu.Lock()
				delete(s.inFlight, taskID)
				s.mu.Unlock()
			}(t.ID)
		}
	}

	if len(firedRecurring) > 0 {
		firedAt := time.UnixMilli(now)
		go func() {
			if err := platformstore.MarkCronTasksFired(s.opts.ProjectRoot, firedRecurring, firedAt); err != nil {
				logger.Warn(fmt.Sprintf("cron scheduler: failed to mark tasks fired: %v", err))
			}
		}()
	}

	for id := range s.nextFireAt {
		if _, ok := seen[id]; !ok {
			delete(s.nextFireAt, id)
		}
	}
}

// cronTaskFromPlatform converts a platform CronTask to the local runtime
// CronTask type.
func cronTaskFromPlatform(t platformstore.CronTask) CronTask {
	return CronTask{
		ID:          t.ID,
		Cron:        t.Cron,
		Prompt:      t.Prompt,
		CreatedAt:   t.CreatedAt,
		LastFiredAt: t.LastFiredAt,
		Recurring:   t.Recurring,
	}
}

// platformTaskFromCron converts a local runtime CronTask back to the platform
// CronTask type.
func platformTaskFromCron(t CronTask) platformstore.CronTask {
	return platformstore.CronTask{
		ID:          t.ID,
		Cron:        t.Cron,
		Prompt:      t.Prompt,
		CreatedAt:   t.CreatedAt,
		LastFiredAt: t.LastFiredAt,
		Recurring:   t.Recurring,
	}
}
