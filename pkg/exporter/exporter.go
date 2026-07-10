// Package exporter mirrors the SQLite source of truth to a beads-compatible
// issues.jsonl on disk (and optionally commits/pushes it via git) as an
// off-machine backup. Writes are atomic (temp + rename) and debounced so a
// burst of mutations produces a single export.
package exporter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// Source provides the tasks to export.
type Source interface {
	All() ([]model.Task, error)
}

// Config configures an Exporter.
type Config struct {
	Path     string        // target issues.jsonl path ("" disables export)
	Interval time.Duration // debounce window (default 15s)
	Git      bool          // git add/commit/push after each export
	GitPush  bool          // include push (requires Git)
	Message  string        // commit message (default "tasks: sync issues.jsonl")
}

// Exporter debounces exports triggered via Notify.
type Exporter struct {
	src  Source
	cfg  Config
	mu   sync.Mutex
	wake chan struct{}
	log  func(string, ...any)
}

// New creates an Exporter. Returns nil if export is disabled (empty Path).
func New(src Source, cfg Config, logf func(string, ...any)) *Exporter {
	if cfg.Path == "" {
		return nil
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 15 * time.Second
	}
	if cfg.Message == "" {
		cfg.Message = "tasks: sync issues.jsonl"
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Exporter{src: src, cfg: cfg, wake: make(chan struct{}, 1), log: logf}
}

// Notify signals that state changed; a coalesced export runs after the debounce
// interval. Safe to call from any goroutine; never blocks.
func (e *Exporter) Notify() {
	if e == nil {
		return
	}
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// Run drives the debounce loop until ctx is cancelled, then flushes once more.
func (e *Exporter) Run(ctx context.Context) {
	if e == nil {
		return
	}
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	pending := false
	for {
		select {
		case <-ctx.Done():
			if pending {
				e.exportNow()
			}
			return
		case <-e.wake:
			if !pending {
				pending = true
				timer.Reset(e.cfg.Interval)
			}
		case <-timer.C:
			pending = false
			e.exportNow()
		}
	}
}

// ExportOnce writes the JSONL immediately (used for one-shot `tasksd export`).
func (e *Exporter) ExportOnce() error {
	if e == nil {
		return nil
	}
	return e.export()
}

func (e *Exporter) exportNow() {
	if err := e.export(); err != nil {
		e.log("export failed: %v", err)
	} else {
		e.log("exported issues.jsonl -> %s", e.cfg.Path)
	}
}

func (e *Exporter) export() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	tasks, err := e.src.All()
	if err != nil {
		return err
	}
	if dir := filepath.Dir(e.cfg.Path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(e.cfg.Path), ".issues-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	if err := model.WriteJSONL(tmp, tasks); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, e.cfg.Path); err != nil {
		return err
	}
	if e.cfg.Git {
		if err := e.gitCommit(); err != nil {
			return fmt.Errorf("git: %w", err)
		}
	}
	return nil
}

func (e *Exporter) gitCommit() error {
	dir := filepath.Dir(e.cfg.Path)
	run := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if _, err := run("add", filepath.Base(e.cfg.Path)); err != nil {
		return err
	}
	// Nothing staged -> skip commit (diff --quiet returns non-zero when changes).
	if _, err := run("diff", "--cached", "--quiet"); err == nil {
		return nil // no changes
	}
	if out, err := run("commit", "-m", e.cfg.Message); err != nil {
		return fmt.Errorf("commit: %s", out)
	}
	if e.cfg.GitPush {
		if out, err := run("push"); err != nil {
			return fmt.Errorf("push: %s", out)
		}
	}
	return nil
}
