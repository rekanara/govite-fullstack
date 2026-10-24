// Package domain is health, status and maintenance cleanup.
package domain

import (
	"context"
	"time"
)

type CleanupResult struct {
	Ran     bool             `json:"ran"`
	Cutoff  *time.Time       `json:"cutoff"`
	Deleted map[string]int64 `json:"deleted"`
}

type Check struct {
	Status    string `json:"status"` // up | down
	LatencyMs *int64 `json:"latencyMs"`
}

// Pinger is a dependency health probe; nil means not configured.
type Pinger func(context.Context) error

// Cleaner purges rows older than cutoff. ran=false when another instance holds the lock.
type Cleaner interface {
	Purge(ctx context.Context, cutoff time.Time) (deleted map[string]int64, ran bool, err error)
}
