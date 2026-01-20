package monitor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/wwwzy/CentAgent/internal/docker"
	"github.com/wwwzy/CentAgent/internal/storage"
)

type LogCollector struct {
	// cfg 为日志收集流水线配置（队列、批量写入、tailer 上限、重连策略等）。
	cfg LogConfig

	// store 为持久化层，负责将解析后的日志写入 SQLite。
	store *storage.Storage

	// logCh 为“解析完成 -> 等待批量落库”的内部队列。
	logCh chan storage.ContainerLog

	// tailers 保存当前正在 Follow 的容器 tailer 取消函数；key 为 containerID。
	tailersMu sync.Mutex
	tailers   map[string]context.CancelFunc
}

func NewLogCollector(store *storage.Storage) (*LogCollector, error) {
	if store == nil {
		return nil, errors.New("storage is required")
	}
	return &LogCollector{store: store}, nil
}

func (c *LogCollector) Run(ctx context.Context) error {
	if c == nil || c.store == nil {
		return errors.New("log collector not initialized")
	}
	c.cfg = c.cfg.withDefaults()
	c.logCh = make(chan storage.ContainerLog, c.cfg.QueueSize)
	c.tailers = make(map[string]context.CancelFunc)

	startedAt := time.Now()
	if !c.cfg.SinceFromStart {
		startedAt = time.Time{}
	}

	writerErrCh := make(chan error, 1)
	go func() {
		writerErrCh <- c.writeLoop(ctx)
	}()

	if err := c.reconcileRunning(ctx, startedAt); err != nil {
		c.cfg.OnError(err)
	}

	eventsErr := c.eventsLoop(ctx, startedAt)
	c.stopAllTailers()

	writerErr := <-writerErrCh
	if writerErr != nil && !errors.Is(writerErr, context.Canceled) {
		return writerErr
	}

	if eventsErr != nil && !errors.Is(eventsErr, context.Canceled) {
		return eventsErr
	}
	return nil
}

func (c *LogCollector) reconcileRunning(ctx context.Context, since time.Time) error {
	items, err := docker.ListContainerDetail(ctx, docker.ListContainersOptions{All: false, Status: "running"})
	if err != nil {
		return err
	}
	for _, it := range items {
		c.startTailer(ctx, it.ID, it.Names, since)
	}
	return nil
}

func (c *LogCollector) eventsLoop(ctx context.Context, startedAt time.Time) error {
	backoff := c.cfg.ReconnectDelay
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		args := filters.NewArgs()
		args.Add("type", "container")
		msgCh, errCh := docker.Events(ctx, events.ListOptions{Filters: args})

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case err, ok := <-errCh:
				if !ok {
					time.Sleep(withJitter(backoff, c.cfg.ReconnectJitter))
					goto reconnect
				}
				if err != nil && !errors.Is(err, context.Canceled) {
					c.cfg.OnError(fmt.Errorf("events stream error: %w", err))
				}
				time.Sleep(withJitter(backoff, c.cfg.ReconnectJitter))
				goto reconnect
			case msg, ok := <-msgCh:
				if !ok {
					time.Sleep(withJitter(backoff, c.cfg.ReconnectJitter))
					goto reconnect
				}
				c.handleEvent(ctx, msg, startedAt)
			}
		}

	reconnect:
		continue
	}
}

func (c *LogCollector) handleEvent(ctx context.Context, msg events.Message, startedAt time.Time) {
	if msg.Type != "container" {
		return
	}
	containerID := msg.Actor.ID
	if containerID == "" {
		return
	}

	action := msg.Action
	switch action {
	case "start":
		since := startedAt
		if !since.IsZero() {
			since = since.Add(-500 * time.Millisecond)
		}
		c.startTailer(ctx, containerID, "", since)
	case "die", "stop", "destroy":
		c.stopTailer(containerID)
	}
}

func (c *LogCollector) startTailer(ctx context.Context, containerID string, name string, since time.Time) {
	c.tailersMu.Lock()
	if _, ok := c.tailers[containerID]; ok {
		c.tailersMu.Unlock()
		return
	}
	if c.cfg.TailerLimit > 0 && len(c.tailers) >= c.cfg.TailerLimit {
		c.tailersMu.Unlock()
		c.cfg.OnError(fmt.Errorf("tailer limit reached: %d", c.cfg.TailerLimit))
		return
	}
	tailerCtx, cancel := context.WithCancel(ctx)
	c.tailers[containerID] = cancel
	c.tailersMu.Unlock()

	go func() {
		defer c.stopTailer(containerID)

		info, err := c.inspectContainer(tailerCtx, containerID)
		if err != nil {
			c.cfg.OnError(err)
			return
		}
		if name == "" {
			name = info.name
		}
		if since.IsZero() && c.cfg.SinceFromStart {
			since = time.Now()
		}
		if err := c.tailContainer(tailerCtx, containerID, name, info.tty, since); err != nil && !errors.Is(err, context.Canceled) {
			c.cfg.OnError(err)
		}
	}()
}

func (c *LogCollector) stopTailer(containerID string) {
	c.tailersMu.Lock()
	cancel, ok := c.tailers[containerID]
	if ok {
		delete(c.tailers, containerID)
	}
	c.tailersMu.Unlock()
	if ok {
		cancel()
	}
}

func (c *LogCollector) stopAllTailers() {
	c.tailersMu.Lock()
	tailers := make([]context.CancelFunc, 0, len(c.tailers))
	for _, cancel := range c.tailers {
		tailers = append(tailers, cancel)
	}
	c.tailers = make(map[string]context.CancelFunc)
	c.tailersMu.Unlock()
	for _, cancel := range tailers {
		cancel()
	}
}

type containerInspectInfo struct {
	name string
	tty  bool
}

func (c *LogCollector) inspectContainer(ctx context.Context, containerID string) (containerInspectInfo, error) {
	inspectCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	info, err := docker.InspectContainerDeatil(inspectCtx, containerID)
	if err != nil {
		return containerInspectInfo{}, fmt.Errorf("inspect container %s: %w", containerID, err)
	}

	tty := false
	if info.Config != nil {
		tty = info.Config.Tty
	}
	return containerInspectInfo{name: info.Name, tty: tty}, nil
}

func (c *LogCollector) tailContainer(ctx context.Context, containerID, containerName string, tty bool, since time.Time) error {
	sinceStr := ""
	if !since.IsZero() {
		sinceStr = since.UTC().Format(time.RFC3339Nano)
	}

	// TODO: 建议在 internal/docker 增加原子能力函数（SubscribeEvents / GetContainerLogsFollow），monitor 只负责调度与落库。
	r, err := docker.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Since:      sinceStr,
	})
	if err != nil {
		return fmt.Errorf("container logs follow %s: %w", containerID, err)
	}
	defer r.Close()
	go func() {
		<-ctx.Done()
		_ = r.Close()
	}()

	if tty {
		return c.scanLines(ctx, "stdout", containerID, containerName, r)
	}

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	defer func() {
		_ = stdoutR.Close()
		_ = stderrR.Close()
	}()

	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		_, _ = stdcopy.StdCopy(stdoutW, stderrW, r)
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	var scanWG sync.WaitGroup
	scanWG.Add(2)
	go func() {
		defer scanWG.Done()
		_ = c.scanLines(ctx, "stdout", containerID, containerName, stdoutR)
	}()
	go func() {
		defer scanWG.Done()
		_ = c.scanLines(ctx, "stderr", containerID, containerName, stderrR)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-copyDone:
		scanWG.Wait()
		return nil
	}
}

func (c *LogCollector) scanLines(ctx context.Context, source, containerID, containerName string, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, c.cfg.MaxLineBytes)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ts, msg := parseDockerTimestampedLine(scanner.Text())
		rec := storage.ContainerLog{
			ContainerID:   containerID,
			ContainerName: containerName,
			Source:        source,
			Level:         inferLogLevel(msg),
			Message:       msg,
			Timestamp:     ts,
			Raw:           scanner.Text(),
		}

		select {
		case c.logCh <- rec:
		default:
			c.cfg.OnError(fmt.Errorf("log queue full"))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan logs (%s/%s): %w", containerID, source, err)
	}
	return nil
}

func (c *LogCollector) writeLoop(ctx context.Context) error {
	flushTicker := time.NewTicker(c.cfg.FlushInterval)
	defer flushTicker.Stop()

	buf := make([]storage.ContainerLog, 0, c.cfg.BatchSize)
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		err := c.store.InsertContainerLogs(ctx, buf)
		buf = buf[:0]
		return err
	}

	for {
		select {
		case <-ctx.Done():
			_ = flush()
			return ctx.Err()
		case rec := <-c.logCh:
			buf = append(buf, rec)
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

func parseDockerTimestampedLine(line string) (time.Time, string) {
	i := strings.IndexByte(line, ' ')
	if i <= 0 {
		return time.Now(), line
	}
	tsStr := line[:i]
	msg := strings.TrimLeft(line[i+1:], " ")
	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		return time.Now(), line
	}
	return ts, msg
}

func inferLogLevel(msg string) string {
	s := strings.TrimSpace(msg)
	if s == "" {
		return ""
	}

	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "error"):
		return "ERROR"
	case strings.HasPrefix(lower, "warn"):
		return "WARN"
	case strings.HasPrefix(lower, "info"):
		return "INFO"
	case strings.HasPrefix(lower, "debug"):
		return "DEBUG"
	case strings.HasPrefix(lower, "fatal"):
		return "FATAL"
	}
	return ""
}

func withJitter(base, jitter time.Duration) time.Duration {
	if base <= 0 {
		base = 2 * time.Second
	}
	if jitter <= 0 {
		return base
	}
	delta := time.Duration(rand.Int64N(int64(jitter)*2+1)) - jitter
	return base + delta
}
