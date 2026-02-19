package delivery

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/shohag/piperelay/internal/config"
	"github.com/shohag/piperelay/internal/storage"
)

type Pool struct {
	store    storage.Storage
	worker   *Worker
	workers  int
	pollRate time.Duration
	log      zerolog.Logger
	stop     chan struct{}
	wg       sync.WaitGroup
}

func NewPool(cfg config.DeliveryConfig, store storage.Storage, log zerolog.Logger) *Pool {
	sender := NewSender(cfg.Timeout)

	schedule := cfg.RetrySchedule
	if len(schedule) == 0 {
		schedule = DefaultRetrySchedule
	}

	worker := NewWorker(store, sender, cfg.MaxAttempts, schedule, log)

	return &Pool{
		store:    store,
		worker:   worker,
		workers:  cfg.Workers,
		pollRate: 1 * time.Second,
		log:      log,
		stop:     make(chan struct{}),
	}
}

func (p *Pool) Start(ctx context.Context) {
	p.log.Info().Int("workers", p.workers).Msg("starting delivery worker pool")

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.pollLoop(ctx)
	}()
}

func (p *Pool) Stop() {
	p.log.Info().Msg("stopping delivery worker pool")
	close(p.stop)
	p.wg.Wait()
	p.log.Info().Msg("delivery worker pool stopped")
}

func (p *Pool) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(p.pollRate)
	defer ticker.Stop()

	sem := make(chan struct{}, p.workers)

	for {
		select {
		case <-p.stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			deliveries, err := p.store.GetPendingDeliveries(ctx, p.workers)
			if err != nil {
				p.log.Error().Err(err).Msg("failed to fetch pending deliveries")
				continue
			}

			for _, d := range deliveries {
				d := d
				sem <- struct{}{}
				p.wg.Add(1)
				go func() {
					defer p.wg.Done()
					defer func() { <-sem }()
					p.worker.Process(ctx, d)
				}()
			}
		}
	}
}
