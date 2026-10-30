package diagnostic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Event is the stored machine record. Codes/stages support localization;
// Message retains technical text rather than becoming a translated UI string.
type Event struct {
	Time        time.Time `json:"time"`
	Level       string    `json:"level"`
	Code        string    `json:"code"`
	Stage       string    `json:"stage,omitempty"`
	Input       string    `json:"input,omitempty"`
	Output      string    `json:"output,omitempty"`
	StreamIndex *int      `json:"streamIndex,omitempty"`
	Attempt     int       `json:"attempt,omitempty"`
	Message     string    `json:"message"`
}

type LogWriter struct{ Recorder *Recorder }

func (w LogWriter) Write(data []byte) (int, error) {
	w.Recorder.Record(Event{Level: "info", Code: "cli_stderr", Message: strings.TrimSpace(string(data))})
	// The CLI checks Recorder.Close's accumulated error before returning.
	return len(data), nil
}

// scope is copied into derived contexts so parallel jobs cannot overwrite each
// other's attribution. A shared sink must itself support concurrent calls.
type scope struct {
	sink        func(Event)
	input       string
	output      string
	stage       string
	attempt     int
	streamIndex *int
}
type scopeKey struct{}

func getScope(ctx context.Context) scope {
	value, _ := ctx.Value(scopeKey{}).(scope)
	return value
}

// WithSink replaces, rather than chains, the sink. Interceptors must explicitly
// forward to the parent context when records should reach the original sink.
func WithSink(ctx context.Context, sink func(Event)) context.Context {
	value := getScope(ctx)
	value.sink = sink
	return context.WithValue(ctx, scopeKey{}, value)
}

func WithJob(ctx context.Context, input, output string, attempt int) context.Context {
	value := getScope(ctx)
	value.input, value.output, value.attempt = input, output, attempt
	return context.WithValue(ctx, scopeKey{}, value)
}

func WithStage(ctx context.Context, stage string) context.Context {
	value := getScope(ctx)
	value.stage = stage
	return context.WithValue(ctx, scopeKey{}, value)
}

func WithStream(ctx context.Context, index int) context.Context {
	value := getScope(ctx)
	value.streamIndex = &index
	return context.WithValue(ctx, scopeKey{}, value)
}

// Emit fills absent fields from context while preserving explicit attribution.
// Delivery is synchronous; callers must not hold locks needed by their sink.
func Emit(ctx context.Context, event Event) {
	value := getScope(ctx)
	if value.sink == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if event.Input == "" {
		event.Input = value.input
	}
	if event.Output == "" {
		event.Output = value.output
	}
	if event.Stage == "" {
		event.Stage = value.stage
	}
	if event.Attempt == 0 {
		event.Attempt = value.attempt
	}
	if event.StreamIndex == nil {
		event.StreamIndex = value.streamIndex
	}
	value.sink(event)
}

// ProgramDirectory keeps logs beside the distributed application, not inside a
// signed macOS bundle. Resolving the executable also handles launcher symlinks.
func ProgramDirectory() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	return programDirectory(executable), nil
}

func programDirectory(executable string) string {
	dir := filepath.Dir(executable)
	if filepath.Base(dir) == "MacOS" && filepath.Base(filepath.Dir(dir)) == "Contents" {
		bundle := filepath.Dir(filepath.Dir(dir))
		if strings.HasSuffix(bundle, ".app") {
			return filepath.Dir(bundle)
		}
	}
	return dir
}

// Recorder serializes rotation, writes, reads and close for one launch. The
// first write failure is sticky so later UI snapshots/CLI exit cannot hide it.
type Recorder struct {
	mu    sync.Mutex
	path  string
	file  *os.File
	size  int64
	limit int64
	err   error
}

func Open(directory string) (*Recorder, error) {
	dir := filepath.Join(directory, "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(dir, "hdma2truehd-"+time.Now().Format("20060102-150405")+"-*.jsonl")
	if err != nil {
		return nil, err
	}
	r := &Recorder{path: file.Name(), file: file, limit: 5 << 20}
	r.Record(Event{Time: time.Now().UTC(), Level: "info", Code: "session_start", Message: "Local diagnostic session; paths may contain personal information. No upload."})
	return r, r.Error()
}

func (r *Recorder) Path() string { return r.path }

// Record retains write failures for the GUI state and CLI exit diagnostics.
// Each session keeps at most two 5 MiB segments, without deleting other sessions.
func (r *Recorder) Record(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return
	}
	if r.file == nil {
		r.err = fmt.Errorf("diagnostic log is closed: %s", r.path)
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	// Marshal escapes embedded message newlines. Rotate between whole records,
	// so each appended physical line still represents one complete event.
	data, err := json.Marshal(event)
	if err == nil && r.size+int64(len(data)+1) > r.limit {
		err = r.file.Close()
		if err == nil {
			previous := r.path + ".1"
			if removeErr := os.Remove(previous); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = removeErr
			}
			if err == nil {
				err = os.Rename(r.path, previous)
			}
			if err == nil {
				r.file, err = os.OpenFile(r.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			}
			r.size = 0
		}
	}
	if err == nil {
		var n int
		n, err = r.file.Write(append(data, '\n'))
		r.size += int64(n)
		if err == nil && n != len(data)+1 {
			err = io.ErrShortWrite
		}
	}
	if err != nil {
		r.err = fmt.Errorf("write diagnostic log %s: %w", r.path, err)
	}
}

func (r *Recorder) Error() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		r.err = errors.Join(r.err, r.file.Sync(), r.file.Close())
		r.file = nil
	}
	return r.err
}

// View carries the raw bounded tail; human-readable formatting belongs to the
// frontend and must not rewrite persisted JSONL or hide a recorder problem.
type View struct {
	Path      string   `json:"path"`
	Content   string   `json:"content"`
	Truncated bool     `json:"truncated"`
	Problem   *Problem `json:"problem,omitempty"`
}

// Read returns a bounded text tail, not necessarily complete JSONL records.
// A sticky write error remains visible in Problem even if old text is readable.
func (r *Recorder) Read() (View, error) {
	// Keep our writes/rotation excluded until the tail offset and read agree.
	r.mu.Lock()
	defer r.mu.Unlock()
	view := View{Path: r.path}
	if r.err != nil {
		view.Problem = &Problem{Code: "log_unavailable", Detail: r.err.Error()}
	}
	file, err := os.Open(r.path)
	if err != nil {
		return view, err
	}
	stat, err := file.Stat()
	if err == nil {
		const limit = 128 << 10
		start := max(int64(0), stat.Size()-limit)
		view.Truncated = start > 0
		_, err = file.Seek(start, io.SeekStart)
		if err == nil {
			var data []byte
			data, err = io.ReadAll(io.LimitReader(file, limit))
			view.Content = string(data)
		}
	}
	return view, errors.Join(err, file.Close())
}
