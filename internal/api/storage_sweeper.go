// Package api — StorageLinkSweeper is the background worker for the
// storage-link TTL flows (plan: storage-links-plan.md Fase 5, todo 7.17.6).
// Each tick:
//
//  1. expired links flagged delete_if_untouched → remove the object (it was
//     never downloaded within its ttl), then flip the row to consumed;
//  2. flip all remaining expired active rows to consumed;
//  3. purge consumed rows older than the retention window.
//
// Started like the other background workers (StartBackgroundWorkers); safe
// to skip in tests — a nil resolver or nil store disables the worker.
package api

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// StorageLinkSweeper periodically enforces link TTLs and purges old rows.
type StorageLinkSweeper struct {
	store     *db.StorageLinkStore
	storage   func() (Storage, error)
	interval  time.Duration
	retention time.Duration // how long consumed rows are kept

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
	// started tracks whether Start actually launched the loop — Stop must
	// not wait on done when the loop never ran (Close() without Start() is
	// the normal test path, mirroring the other workers' no-op semantics).
	started atomic.Bool
}

// NewStorageLinkSweeper creates a sweeper. storage may be nil (rows are
// flipped but objects are never removed); interval <= 0 defaults to 1 minute.
func NewStorageLinkSweeper(store *db.StorageLinkStore, storage func() (Storage, error), interval time.Duration) *StorageLinkSweeper {
	if interval <= 0 {
		interval = time.Minute
	}
	return &StorageLinkSweeper{
		store:     store,
		storage:   storage,
		interval:  interval,
		retention: 7 * 24 * time.Hour,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start runs the sweep loop until Stop. A second Start is a no-op; a Start
// after Stop exits immediately (stop is already closed).
func (s *StorageLinkSweeper) Start(ctx context.Context) {
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				s.sweep()
			}
		}
	}()
}

// Stop terminates the loop. Safe to call before Start (no-op wait) and
// multiple times.
func (s *StorageLinkSweeper) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
		if s.started.Load() {
			<-s.done
		}
	})
}

// sweep runs one pass. Errors are logged, never fatal — the next tick retries.
func (s *StorageLinkSweeper) sweep() {
	if s.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC()

	// 1. TTL objects: expired + untouched → delete the object first, then
	// the row flip below retires the link.
	expired, err := s.store.ListExpired(ctx, now, 200)
	if err != nil {
		log.Printf("formspec: storage link sweep: list expired: %v", err)
		return
	}
	var toDelete []string
	for _, row := range expired {
		if row.DeleteIfUntouched && row.DownloadedAt == "" {
			toDelete = append(toDelete, row.Path)
		}
	}
	if len(toDelete) > 0 && s.storage != nil {
		if store, err := s.storage(); err == nil {
			if del, ok := store.(interface {
				Delete(ctx context.Context, path string) error
			}); ok {
				for _, path := range toDelete {
					if err := del.Delete(ctx, path); err != nil {
						log.Printf("formspec: storage link sweep: delete %s: %v", path, err)
					}
				}
			}
		}
	}

	// 2. Flip all expired active rows to consumed.
	if n, err := s.store.SweepExpired(ctx, now); err != nil {
		log.Printf("formspec: storage link sweep: flip expired: %v", err)
	} else if n > 0 {
		log.Printf("formspec: storage link sweep: expired %d link(s)", n)
	}

	// 3. Purge old consumed rows.
	if _, err := s.store.PurgeOld(ctx, now.Add(-s.retention)); err != nil {
		log.Printf("formspec: storage link sweep: purge: %v", err)
	}
}
