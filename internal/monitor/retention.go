package monitor

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wwwzy/CentAgent/internal/storage"
)

type RetentionCollector struct {
	cfg RetentionConfig

	store *storage.Storage
}

func NewRetentionCollector(store *storage.Storage) (*RetentionCollector, error) {
	if store == nil {
		return nil, errors.New("storage is required")
	}
	return &RetentionCollector{store: store}, nil
}

func (c *RetentionCollector) Run(ctx context.Context) error {
	if c == nil || c.store == nil {
		return errors.New("retention collector not initialized")
	}
	c.cfg = c.cfg.withDefaults()

	if err := c.runOnce(ctx, time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := c.runOnce(ctx, time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
}

func (c *RetentionCollector) runOnce(ctx context.Context, now time.Time) error {
	if c == nil || c.store == nil {
		return errors.New("retention collector not initialized")
	}

	var tasks []func(context.Context) error

	statsCutAll := now.Add(-c.cfg.Stats.KeepAll)
	statsCutAnomaly := now.Add(-c.cfg.Stats.KeepAnomalyUntil)
	tasks = append(tasks, func(ctx context.Context) error {
		return c.deleteStatsBefore(ctx, statsCutAnomaly)
	})
	tasks = append(tasks, func(ctx context.Context) error {
		return c.deleteStatsNonAnomalyInRange(ctx, statsCutAnomaly, statsCutAll)
	})

	logsCutAll := now.Add(-c.cfg.Logs.KeepAll)
	logsCutImportant := now.Add(-c.cfg.Logs.KeepImportantUntil)
	tasks = append(tasks, func(ctx context.Context) error {
		return c.deleteLogsBefore(ctx, logsCutImportant)
	})
	tasks = append(tasks, func(ctx context.Context) error {
		return c.deleteLogsUnimportantInRange(ctx, logsCutImportant, logsCutAll)
	})

	workers := c.cfg.Workers
	if workers > len(tasks) {
		workers = len(tasks)
	}
	if workers <= 0 {
		workers = 1
	}

	jobs := make(chan func(context.Context) error)
	errs := make(chan error, len(tasks))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := job(ctx); err != nil && !errors.Is(err, context.Canceled) {
					errs <- err
				}
			}
		}()
	}

	for _, t := range tasks {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			close(errs)
			return ctx.Err()
		case jobs <- t:
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			c.cfg.OnError(err)
			return err
		}
	}
	return nil
}

func (c *RetentionCollector) deleteStatsBefore(ctx context.Context, before time.Time) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		affected, err := c.store.DeleteContainerStatsBeforeLimited(ctx, before, c.cfg.BatchRows)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := c.sleepIdle(ctx); err != nil {
			return err
		}
	}
}

func (c *RetentionCollector) deleteStatsNonAnomalyInRange(ctx context.Context, from time.Time, to time.Time) error {
	if !to.After(from) {
		return nil
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		affected, err := c.store.DeleteContainerStatsNonAnomalyInRangeLimited(ctx, from, to, c.cfg.Stats.CPUHigh, c.cfg.Stats.MemHigh, c.cfg.BatchRows)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := c.sleepIdle(ctx); err != nil {
			return err
		}
	}
}

func (c *RetentionCollector) deleteLogsBefore(ctx context.Context, before time.Time) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		affected, err := c.store.DeleteContainerLogsBeforeLimited(ctx, before, c.cfg.BatchRows)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := c.sleepIdle(ctx); err != nil {
			return err
		}
	}
}

func (c *RetentionCollector) deleteLogsUnimportantInRange(ctx context.Context, from time.Time, to time.Time) error {
	if !to.After(from) {
		return nil
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		affected, err := c.store.DeleteContainerLogsUnimportantInRangeLimited(ctx, from, to, c.cfg.Logs.KeepLevels, c.cfg.Logs.KeepSources, c.cfg.BatchRows)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := c.sleepIdle(ctx); err != nil {
			return err
		}
	}
}

func (c *RetentionCollector) sleepIdle(ctx context.Context) error {
	if c.cfg.IdleSleep <= 0 {
		return nil
	}
	timer := time.NewTimer(c.cfg.IdleSleep)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
