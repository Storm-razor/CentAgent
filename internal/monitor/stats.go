package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/wwwzy/CentAgent/internal/docker"
	"github.com/wwwzy/CentAgent/internal/storage"
)

type containerMeta struct {
	ID   string
	Name string
}

type listContainersFunc func(ctx context.Context) ([]containerMeta, error)
type fetchStatsFunc func(ctx context.Context, c containerMeta) (storage.ContainerStat, error)

type StatsCollector struct {
	cfg StatsConfig

	store *storage.Storage

	list  listContainersFunc
	fetch fetchStatsFunc
}

func NewStatsCollector(store *storage.Storage) (*StatsCollector, error) {
	if store == nil {
		return nil, errors.New("storage is required")
	}
	return &StatsCollector{
		store: store,
	}, nil
}

func (c *StatsCollector) WithLister(fn listContainersFunc) *StatsCollector {
	c.list = fn
	return c
}

func (c *StatsCollector) WithFetcher(fn fetchStatsFunc) *StatsCollector {
	c.fetch = fn
	return c
}

func (c *StatsCollector) Run(ctx context.Context) error {
	if c == nil || c.store == nil {
		return errors.New("stats collector not initialized")
	}
	c.cfg = c.cfg.withDefaults()

	listFn := c.list
	if listFn == nil {
		listFn = c.defaultListContainers
	}
	fetchFn := c.fetch
	if fetchFn == nil {
		fetchFn = c.defaultFetchStats
	}

	jobs := make(chan containerMeta, c.cfg.QueueSize)
	results := make(chan storage.ContainerStat, c.cfg.QueueSize)

	var workersWG sync.WaitGroup
	for i := 0; i < c.cfg.Workers; i++ {
		workersWG.Add(1)
		go func() {
			defer workersWG.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					stat, err := fetchFn(ctx, job)
					if err != nil {
						c.cfg.OnError(err)
						continue
					}
					select {
					case <-ctx.Done():
						return
					case results <- stat:
					}
				}
			}
		}()
	}

	writerDone := make(chan struct{})
	var writerErr error
	go func() {
		defer close(writerDone)
		writerErr = c.writeLoop(ctx, results)
	}()

	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	c.enqueueOnce(ctx, listFn, jobs)

	for {
		select {
		case <-ctx.Done():
			close(jobs)
			workersWG.Wait()
			close(results)
			<-writerDone
			if errors.Is(writerErr, context.Canceled) {
				return nil
			}
			return writerErr
		case <-ticker.C:
			c.enqueueOnce(ctx, listFn, jobs)
		}
	}
}

func (c *StatsCollector) enqueueOnce(ctx context.Context, listFn listContainersFunc, jobs chan<- containerMeta) {
	containers, err := listFn(ctx)
	if err != nil {
		c.cfg.OnError(err)
		return
	}

	for _, meta := range containers {
		select {
		case <-ctx.Done():
			return
		case jobs <- meta:
		}
	}
}

func (c *StatsCollector) writeLoop(ctx context.Context, results <-chan storage.ContainerStat) error {
	flushTicker := time.NewTicker(c.cfg.FlushInterval)
	defer flushTicker.Stop()

	buf := make([]storage.ContainerStat, 0, c.cfg.BatchSize)
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		err := c.store.InsertContainerStats(ctx, buf)
		buf = buf[:0]
		return err
	}

	for {
		select {
		case <-ctx.Done():
			_ = flush()
			return ctx.Err()
		case stat, ok := <-results:
			if !ok {
				return flush()
			}
			buf = append(buf, stat)
			if len(buf) >= c.cfg.BatchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		case <-flushTicker.C:
			if err := flush(); err != nil {
				return err
			}
		}
	}
}

func (c *StatsCollector) defaultListContainers(ctx context.Context) ([]containerMeta, error) {
	containers, err := docker.ListContainers(ctx, docker.ListContainersOptions{All: false})
	if err != nil {
		return nil, err
	}

	out := make([]containerMeta, 0, len(containers))
	for _, item := range containers {
		out = append(out, containerMeta{
			ID:   item.ID,
			Name: item.Names,
		})
	}
	return out, nil
}

func (c *StatsCollector) defaultFetchStats(ctx context.Context, meta containerMeta) (storage.ContainerStat, error) {
	resp, err := docker.GetContainerStatsOneShot(ctx, meta.ID)
	if err != nil {
		return storage.ContainerStat{}, err
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&stats); err != nil {
		return storage.ContainerStat{}, err
	}

	rawJSON, _ := json.Marshal(stats)
	if c.cfg.MaxRawJSONBytes > 0 && len(rawJSON) > c.cfg.MaxRawJSONBytes {
		rawJSON = []byte(`{"_truncated":true}`)
	}

	cpuPercent := calculateCPUPercent(stats)
	memUsage := uint64(stats.MemoryStats.Usage)
	memLimit := uint64(stats.MemoryStats.Limit)
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (float64(memUsage) / float64(memLimit)) * 100.0
	}

	var netRx, netTx uint64
	for _, nw := range stats.Networks {
		netRx += uint64(nw.RxBytes)
		netTx += uint64(nw.TxBytes)
	}

	var blkRead, blkWrite uint64
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			blkRead += uint64(entry.Value)
		case "write":
			blkWrite += uint64(entry.Value)
		}
	}

	collectedAt := time.Now()
	if !stats.Read.IsZero() {
		collectedAt = stats.Read
	}

	return storage.ContainerStat{
		ContainerID:     meta.ID,
		ContainerName:   meta.Name,
		CPUPercent:      cpuPercent,
		MemUsageBytes:   memUsage,
		MemLimitBytes:   memLimit,
		MemPercent:      memPercent,
		NetRxBytes:      netRx,
		NetTxBytes:      netTx,
		BlockReadBytes:  blkRead,
		BlockWriteBytes: blkWrite,
		Pids:            uint64(stats.PidsStats.Current),
		RawJSON:         string(rawJSON),
		CollectedAt:     collectedAt,
	}, nil
}

func calculateCPUPercent(stats container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	if cpuDelta <= 0 || systemDelta <= 0 {
		return 0
	}
	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs <= 0 {
		if n := len(stats.CPUStats.CPUUsage.PercpuUsage); n > 0 {
			onlineCPUs = float64(n)
		} else {
			onlineCPUs = 1
		}
	}
	return (cpuDelta / systemDelta) * onlineCPUs * 100.0
}
