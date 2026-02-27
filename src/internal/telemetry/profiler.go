package telemetry

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

type SessionInfo struct {
	LogPath  string
	CPUPath  string
	HeapPath string
}

type session struct {
	startedAt time.Time
	info      SessionInfo
	logFile   *os.File
	cpuFile   *os.File
	logger    *slog.Logger
}

var (
	mu     sync.RWMutex
	active *session
)

func Start(profileDir string) (SessionInfo, error) {
	mu.Lock()
	defer mu.Unlock()

	if active != nil {
		return active.info, nil
	}

	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return SessionInfo{}, err
	}

	stamp := time.Now().UTC().Format("20060102-150405.000")
	info := SessionInfo{
		LogPath:  filepath.Join(profileDir, fmt.Sprintf("trace-%s.jsonl", stamp)),
		CPUPath:  filepath.Join(profileDir, fmt.Sprintf("cpu-%s.pprof", stamp)),
		HeapPath: filepath.Join(profileDir, fmt.Sprintf("heap-%s.pprof", stamp)),
	}

	logFile, err := os.Create(info.LogPath)
	if err != nil {
		return SessionInfo{}, err
	}

	cpuFile, err := os.Create(info.CPUPath)
	if err != nil {
		_ = logFile.Close()
		return SessionInfo{}, err
	}

	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		_ = cpuFile.Close()
		_ = logFile.Close()
		return SessionInfo{}, err
	}

	logger := slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo}))
	active = &session{
		startedAt: time.Now(),
		info:      info,
		logFile:   logFile,
		cpuFile:   cpuFile,
		logger:    logger,
	}
	logger.Info(
		"profile.session_start",
		"profile_dir", profileDir,
		"log_path", info.LogPath,
		"cpu_profile_path", info.CPUPath,
		"heap_profile_path", info.HeapPath,
		"pid", os.Getpid(),
		"goos", runtime.GOOS,
		"goarch", runtime.GOARCH,
	)
	return info, nil
}

func Stop() (SessionInfo, error) {
	mu.Lock()
	s := active
	active = nil
	mu.Unlock()

	if s == nil {
		return SessionInfo{}, nil
	}

	pprof.StopCPUProfile()

	var firstErr error
	if err := s.cpuFile.Close(); err != nil {
		firstErr = err
	}

	runtime.GC()
	heapFile, err := os.Create(s.info.HeapPath)
	if err != nil && firstErr == nil {
		firstErr = err
	}
	if err == nil {
		if writeErr := pprof.WriteHeapProfile(heapFile); writeErr != nil && firstErr == nil {
			firstErr = writeErr
		}
		if closeErr := heapFile.Close(); closeErr != nil && firstErr == nil {
			firstErr = closeErr
		}
	}

	s.logger.Info("profile.session_stop", "elapsed_ms", time.Since(s.startedAt).Milliseconds())
	if err := s.logFile.Close(); err != nil && firstErr == nil {
		firstErr = err
	}

	return s.info, firstErr
}

func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return active != nil
}

func Event(name string, kv ...any) {
	mu.RLock()
	s := active
	mu.RUnlock()
	if s == nil {
		return
	}
	s.logger.Info(name, normalizeKV(kv)...)
}

func StartSpan(name string, kv ...any) func(kv ...any) {
	if !Enabled() {
		return func(...any) {}
	}
	started := time.Now()
	Event(name+".start", kv...)
	return func(doneKV ...any) {
		fields := make([]any, 0, len(kv)+len(doneKV)+2)
		fields = append(fields, kv...)
		fields = append(fields, doneKV...)
		fields = append(fields, "duration_ms", time.Since(started).Milliseconds())
		Event(name+".done", fields...)
	}
}

func normalizeKV(kv []any) []any {
	if len(kv)%2 == 0 {
		return kv
	}
	out := make([]any, len(kv)+1)
	copy(out, kv)
	out[len(out)-1] = "(missing)"
	return out
}
