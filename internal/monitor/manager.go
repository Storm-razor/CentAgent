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
	logs  *LogCollector

	started atomic.Bool

	cancel context.CancelFunc
	wg     sync.WaitGroup

	runErrMu sync.Mutex
	runErr   error
}

func NewManager(cfg Config) (*Manager, error) {
	cfg.Stats = cfg.Stats.withDefaults()
	cfg.Logs = cfg.Logs.withDefaults()
	return &Manager{
		cfg:   cfg,
		stats: nil,
		logs:  nil,
	}, nil
}
func (m *Manager) WithStats(stats *StatsCollector) *Manager {
	if m == nil {
		return nil
	}
	m.stats = stats
	if m.stats != nil {
		m.stats.cfg = m.cfg.Stats
	}
	return m
}

func (m *Manager) WithLogs(logs *LogCollector) *Manager {
	if m == nil {
		return nil
	}
	m.logs = logs
	if m.logs != nil {
		m.logs.cfg = m.cfg.Logs
	}
	return m
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
		if m.stats == nil {
			m.cancel()
			return errors.New("stats collector is required when stats enabled")
		}
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

	if m.cfg.Logs.Enabled {
		if m.logs == nil {
			m.cancel()
			return errors.New("logs collector is required when logs enabled")
		}
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			if err := m.logs.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
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
