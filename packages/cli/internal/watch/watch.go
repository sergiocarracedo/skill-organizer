package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	loggingpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/logging"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type Runner struct {
	registryPath string
	watcher      *fsnotify.Watcher
	mu           sync.Mutex
	pending      map[string]time.Time
	ignoredUntil map[string]time.Time
	locations    map[string]configpkg.Location
	watchPaths   map[string]struct{}
	logger       loggingpkg.Logger
}

func New(registryPath string, logger loggingpkg.Logger) (*Runner, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	if logger == nil {
		logger = loggingpkg.NewStd(configpkg.DefaultLogLevel)
	}

	return &Runner{
		registryPath: registryPath,
		watcher:      watcher,
		pending:      map[string]time.Time{},
		ignoredUntil: map[string]time.Time{},
		locations:    map[string]configpkg.Location{},
		watchPaths:   map[string]struct{}{},
		logger:       logger,
	}, nil
}

func (r *Runner) Close() error {
	return r.watcher.Close()
}

func (r *Runner) Run() error {
	r.logger.Infof("watch runner starting")
	if err := r.reloadWatchedProjects(); err != nil {
		r.logger.Errorf("failed to reload watched projects: %v", err)
		return err
	}

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-r.watcher.Events:
			if !ok {
				r.logger.Infof("watcher events channel closed")
				return nil
			}
			r.logger.Debugf("fs event: %s op=%s", event.Name, event.Op.String())
			if filepath.Clean(event.Name) == filepath.Clean(r.registryPath) {
				if err := r.reloadWatchedProjects(); err != nil {
					r.logger.Errorf("failed to reload watched projects after config change: %v", err)
					return err
				}
				continue
			}
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = r.watchDirRecursive(event.Name)
				}
			}
			if r.shouldIgnore(event.Name) {
				r.logger.Debugf("ignored event under target churn suppression: %s", event.Name)
				continue
			}
			r.enqueue(event.Name)
		case err, ok := <-r.watcher.Errors:
			if !ok {
				r.logger.Infof("watcher errors channel closed")
				return nil
			}
			r.logger.Errorf("watcher error: %v", err)
			return fmt.Errorf("watcher error: %w", err)
		case <-ticker.C:
			if err := r.flush(); err != nil {
				r.logger.Errorf("watch flush failed: %v", err)
				return err
			}
		}
	}
}

func (r *Runner) enqueue(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending[path] = time.Now()
}

func (r *Runner) flush() error {
	r.mu.Lock()
	triggered := make([]string, 0, len(r.pending))
	for path, at := range r.pending {
		if time.Since(at) < 300*time.Millisecond {
			continue
		}
		triggered = append(triggered, path)
		delete(r.pending, path)
	}
	r.mu.Unlock()

	if len(triggered) == 0 {
		return nil
	}
	r.logger.Debugf("flushing %d pending paths", len(triggered))

	affected := r.affectedProjects(triggered)
	r.logger.Infof("detected changes affecting %d project(s)", len(affected))
	for configPath, location := range affected {
		r.markIgnored(location.Target, 2*time.Second)
		r.logger.Infof("syncing watched project: %s", configPath)
		result, err := syncpkg.Run(location)
		if err != nil {
			r.logger.Errorf("sync failed for %s: %v", configPath, err)
			return fmt.Errorf("sync watched project %s: %w", configPath, err)
		}
		r.logger.Infof("sync complete for %s: enabled=%d disabled=%d created=%d updated=%d removed=%d", configPath, len(result.Enabled), len(result.Disabled), len(result.Created), len(result.Updated), len(result.Removed))
	}

	return nil
}

func (r *Runner) reloadWatchedProjects() error {
	registry, err := configpkg.LoadRegistryOrEmpty(r.registryPath)
	if err != nil {
		return err
	}
	r.logger.Infof("reloading watched projects from %s", r.registryPath)

	wantedPaths := map[string]struct{}{}
	wantedPaths[filepath.Clean(r.registryPath)] = struct{}{}

	nextLocations := map[string]configpkg.Location{}

	for _, configPath := range registry.Watched {
		location, err := configpkg.LoadLocation(configPath)
		if err != nil {
			r.logger.Errorf("failed to load watched project %s: %v", configPath, err)
			return err
		}
		nextLocations[filepath.Clean(configPath)] = location
		wantedPaths[filepath.Clean(configPath)] = struct{}{}

		sourcePaths, err := collectDirWatchPaths(location.Source)
		if err != nil {
			return fmt.Errorf("collect source watch paths for %q: %w", location.Source, err)
		}
		for _, path := range sourcePaths {
			wantedPaths[path] = struct{}{}
		}

		targetPaths, err := collectDirWatchPaths(location.Target)
		if err != nil {
			return fmt.Errorf("collect target watch paths for %q: %w", location.Target, err)
		}
		for _, path := range targetPaths {
			wantedPaths[path] = struct{}{}
		}
	}

	r.mu.Lock()
	currentPaths := make([]string, 0, len(r.watchPaths))
	for path := range r.watchPaths {
		currentPaths = append(currentPaths, path)
	}
	r.locations = nextLocations
	r.mu.Unlock()

	for _, path := range currentPaths {
		if _, ok := wantedPaths[path]; ok {
			continue
		}
		if err := r.watcher.Remove(path); err != nil {
			return fmt.Errorf("remove watcher for %q: %w", path, err)
		}
		r.mu.Lock()
		delete(r.watchPaths, path)
		r.mu.Unlock()
	}

	for path := range wantedPaths {
		if err := r.ensureWatchPath(path); err != nil {
			return err
		}
	}
	r.logger.Infof("watching %d project(s) across %d path(s)", len(nextLocations), len(wantedPaths))

	return nil
}

func (r *Runner) watchDirRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if err := r.ensureWatchPath(path); err != nil {
			return err
		}
		return nil
	})
}

func (r *Runner) shouldIgnore(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for prefix, until := range r.ignoredUntil {
		if now.After(until) {
			delete(r.ignoredUntil, prefix)
			continue
		}
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

func (r *Runner) markIgnored(path string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ignoredUntil[path] = time.Now().Add(duration)
}

func (r *Runner) affectedProjects(paths []string) map[string]configpkg.Location {
	r.mu.Lock()
	defer r.mu.Unlock()

	affected := make(map[string]configpkg.Location)
	for configPath, location := range r.locations {
		for _, path := range paths {
			cleanPath := filepath.Clean(path)
			if cleanPath == filepath.Clean(configPath) || strings.HasPrefix(cleanPath, filepath.Clean(location.Source)) || strings.HasPrefix(cleanPath, filepath.Clean(location.Target)) {
				affected[configPath] = location
				break
			}
		}
	}

	return affected
}

func (r *Runner) ensureWatchPath(path string) error {
	cleanPath := filepath.Clean(path)

	r.mu.Lock()
	_, ok := r.watchPaths[cleanPath]
	r.mu.Unlock()
	if ok {
		return nil
	}

	if err := r.watcher.Add(cleanPath); err != nil && !os.IsNotExist(err) {
		r.logger.Errorf("failed to watch path %s: %v", cleanPath, err)
		return fmt.Errorf("watch path %q: %w", cleanPath, err)
	}

	r.mu.Lock()
	r.watchPaths[cleanPath] = struct{}{}
	r.mu.Unlock()
	r.logger.Debugf("watching path: %s", cleanPath)
	return nil
}

func collectDirWatchPaths(root string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		paths = append(paths, filepath.Clean(path))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}
