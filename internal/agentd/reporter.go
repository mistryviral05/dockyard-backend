package agentd

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const collectTimeout = 10 * time.Second

type Reporter struct {
	collector Collector
	sender    HeartbeatSender
	log       *slog.Logger
	interval  time.Duration

	failures int
	lastErr  string
}

func NewReporter(collector Collector, sender HeartbeatSender, log *slog.Logger, interval time.Duration) *Reporter {
	return &Reporter{
		collector: collector,
		sender:    sender,
		log:       log.With(slog.String("component", "reporter")),
		interval:  interval,
	}
}

func (r *Reporter) Run(ctx context.Context) {
	r.log.Info("metrics reporter started", slog.Duration("interval", r.interval))

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.cycle(ctx)

	for {
		select {
		case <-ctx.Done():
			r.log.Info("metrics reporter stopped")
			return
		case <-ticker.C:
			r.cycle(ctx)
		}
	}
}

func (r *Reporter) cycle(ctx context.Context) {
	cycleCtx, cancel := context.WithTimeout(ctx, r.interval)
	defer cancel()

	snapshot, err := r.collect(cycleCtx)
	if snapshot == nil {
		if err != nil && ctx.Err() == nil {
			r.recordFailure("collect metrics", err)
		}
		return
	}

	if err := r.sender.SendHeartbeat(cycleCtx, snapshot); err != nil {
		if ctx.Err() != nil {
			return
		}
		r.recordFailure("send heartbeat", err)
		return
	}

	r.recordSuccess(snapshot)
}

func (r *Reporter) collect(ctx context.Context) (*Snapshot, error) {
	collectCtx, cancel := context.WithTimeout(ctx, collectTimeout)
	defer cancel()

	snapshot, err := r.collector.Collect(collectCtx)
	if err != nil && snapshot != nil {
		r.log.Warn("some metrics could not be read",
			slog.String("error", err.Error()),
		)
		return snapshot, nil
	}

	return snapshot, err
}

func (r *Reporter) recordSuccess(s *Snapshot) {
	if r.failures > 0 {
		r.log.Info("heartbeat recovered",
			slog.Int("failed_attempts", r.failures),
		)
	}

	r.failures = 0
	r.lastErr = ""

	r.log.Debug("heartbeat sent",
		slog.Any("cpu_usage", s.CPUUsage),
		slog.Any("ram_usage", s.MemoryUsage),
		slog.Any("disk_usage", s.DiskUsage),
		slog.Any("load_1", s.Load1),
	)
}

func (r *Reporter) recordFailure(stage string, err error) {
	repeated := err.Error() == r.lastErr

	r.failures++
	r.lastErr = err.Error()

	attrs := []any{
		slog.String("stage", stage),
		slog.String("error", err.Error()),
		slog.Int("consecutive_failures", r.failures),
	}

	switch {
	case repeated:
		r.log.Debug("heartbeat still failing", attrs...)
	case errors.Is(err, ErrUnauthorized):
		r.log.Error("agent token is not accepted, fix the token and restart the agent", attrs...)
	default:
		r.log.Error("heartbeat failed", attrs...)
	}
}
