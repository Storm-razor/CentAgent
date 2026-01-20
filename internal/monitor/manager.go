package monitor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

type Manager struct {
	cfg Config

	stats *StatsCollector

	started atomic.Bool

	cancel context.CancelFunc
	wg     sync.WaitGroup

	runErrMu sync.Mutex
	runErr   error
}

func NewManager(cfg Config, stats *StatsCollector) (*Manager, error) {
	if stats == nil {
		return nil, errors.New("stats collector is required")
	}
	cfg.Stats = cfg.Stats.withDefaults()
	stats.cfg = cfg.Stats

	return &Manager{
		cfg:   cfg,
		stats: stats,
	}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	if m == nil {
		return errors.New("manager is nil")
	}
	if !m.started.CompareAndSwap(false, true) {
		return errors.New("manager already started")
	}

	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	if m.cfg.Stats.Enabled {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			if err := m.stats.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				m.runErrMu.Lock()
				if m.runErr == nil {
					m.runErr = err
				}
				m.runErrMu.Unlock()
				m.cancel()
			}
		}()
	}

	return nil
}

func (m *Manager) Stop() {
	if m == nil || m.cancel == nil {
		return
	}
	m.cancel()
}

func (m *Manager) Wait() error {
	if m == nil {
		return nil
	}
	m.wg.Wait()
	m.runErrMu.Lock()
	defer m.runErrMu.Unlock()
	return m.runErr
}

