// Command tasksd is the single-source-of-truth task server: it serves the
// browser UI, a REST API and an MCP endpoint over one HTTP listener (front it
// with `tailscale funnel` for remote access). SQLite is the store.
//
// Usage:
//
//	tasksd [flags]                 # serve (config via flags / TASKS_* env)
//	tasksd import <issues.jsonl>   # import a beads export into the db, then exit
//	tasksd export --out file       # export the db to JSONL, then exit
//	tasksd version                 # print build info
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/tristanMatthias/tasks/internal/config"
	"github.com/tristanMatthias/tasks/pkg/buildinfo"
	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/exporter"
	"github.com/tristanMatthias/tasks/pkg/httpapi"
	"github.com/tristanMatthias/tasks/pkg/importer"
	"github.com/tristanMatthias/tasks/pkg/mcpsrv"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "import":
			runImport(os.Args[2:])
			return
		case "export":
			runExport(os.Args[2:])
			return
		case "version", "--version", "-v":
			fmt.Println(buildinfo.String())
			return
		}
	}
	if err := serve(os.Args[1:], os.Getenv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "tasksd:", err)
		os.Exit(1)
	}
}

func runExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	db := fs.String("db", defaultDB(), "SQLite database path")
	out := fs.String("out", "", "issues.jsonl output path")
	git := fs.Bool("git", false, "git add+commit the export")
	push := fs.Bool("git-push", false, "also push")
	fs.Parse(args)
	if *out == "" {
		fatal("usage: tasksd export --db path --out issues.jsonl [--git] [--git-push]")
	}
	st := mustOpen(*db)
	defer st.Close()
	c, err := core.New(st, core.Options{})
	if err != nil {
		fatal("core: %v", err)
	}
	exp := exporter.New(c, exporter.Config{Path: *out, Git: *git, GitPush: *push}, nil)
	if err := exp.ExportOnce(); err != nil {
		fatal("export: %v", err)
	}
	fmt.Printf("exported -> %s\n", *out)
}

func runImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	db := fs.String("db", defaultDB(), "SQLite database path")
	fs.Parse(args)
	if fs.NArg() < 1 {
		fatal("usage: tasksd import [--db path] <issues.jsonl>")
	}
	st := mustOpen(*db)
	defer st.Close()
	n, err := importer.ImportFile(st, fs.Arg(0))
	if err != nil {
		fatal("import: %v", err)
	}
	fmt.Printf("imported %d tasks into %s\n", n, *db)
}

func serve(args []string, getenv func(string) string) error {
	cfg, err := config.Load(args, getenv, os.Stderr)
	if err != nil {
		return err
	}

	logger := cfg.NewLogger(os.Stderr)
	slog.SetDefault(logger)

	st := mustOpen(cfg.DB)
	defer st.Close()

	// Optional first-run import.
	if cfg.Import != "" {
		if n, _ := st.Count(); n == 0 {
			imported, err := importer.ImportFile(st, cfg.Import)
			if err != nil {
				return fmt.Errorf("startup import: %w", err)
			}
			logger.Info("imported tasks on startup", "count", imported, "source", cfg.Import)
		}
	}

	c, err := core.New(st, core.Options{Prefix: cfg.Prefix, Actor: cfg.Actor})
	if err != nil {
		return err
	}

	srv := httpapi.New(httpapi.Config{
		Core:         c,
		Static:       web.Static(),
		Token:        cfg.Token,
		APIKeys:      true, // bots/agents authenticate with `tasks_<secret>` keys
		MCP:          mcpsrv.Handler(c),
		Logger:       logger,
		MaxBodyBytes: cfg.MaxBodyBytes,
		RateLimit:    cfg.RateLimit,
		RateBurst:    cfg.RateBurst,
		BehindProxy:  cfg.BehindProxy,
		CORSOrigins:  cfg.CORSOrigins,
		Metrics:      cfg.Metrics,
	})

	// Exporter: mirror SQLite -> issues.jsonl (+ optional git) on change.
	exp := exporter.New(c, exporter.Config{
		Path: cfg.Export, Interval: cfg.ExportInterval, Git: cfg.Git, GitPush: cfg.GitPush,
	}, func(format string, a ...any) { logger.Info(fmt.Sprintf(format, a...)) })
	c.SetOnChange(func() {
		srv.Touch()
		exp.Notify()
	})

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	if !cfg.AuthEnabled() {
		logger.Warn("auth is DISABLED (no token set) — do not expose this bind publicly")
	}
	n, _ := st.Count()
	logger.Info("tasksd starting",
		"version", buildinfo.Short(), "addr", cfg.Addr, "tasks", n,
		"prefix", c.Prefix(), "auth", cfg.AuthEnabled(),
		"rate_limit", cfg.RateLimit, "metrics", cfg.Metrics, "log_format", cfg.LogFormat)

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if exp != nil {
		go exp.Run(ctx)
		logger.Info("exporting on change", "path", cfg.Export, "git", cfg.Git, "push", cfg.GitPush)
	}
	errc := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case err := <-errc:
		return fmt.Errorf("listen: %w", err)
	}
	shutCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return httpSrv.Shutdown(shutCtx)
}

func mustOpen(db string) *store.Store {
	if dir := filepath.Dir(db); dir != "" {
		os.MkdirAll(dir, 0o755)
	}
	st, err := store.Open(db)
	if err != nil {
		fatal("open db %s: %v", db, err)
	}
	return st
}

func defaultDB() string {
	if v := os.Getenv("TASKS_DB"); v != "" {
		return v
	}
	return filepath.Join("data", "tasks.db")
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "tasksd: "+format+"\n", a...)
	os.Exit(1)
}
