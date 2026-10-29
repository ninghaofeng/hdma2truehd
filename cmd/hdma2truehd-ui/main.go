//go:build desktop && (darwin || windows)

package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"runtime"
	"sync/atomic"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/av"
	"github.com/ninghaofeng/hdma2truehd/internal/buildinfo"
	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
	"github.com/ninghaofeng/hdma2truehd/internal/gui"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend/*
var frontend embed.FS

// Only these public methods are exposed to the local webview. Lifecycle
// contexts and cancellation functions never become JavaScript bindings.
type App struct {
	ctx     context.Context
	stop    context.CancelFunc
	service *gui.Service
	closing atomic.Bool
}

func (a *App) Inspect(paths []string) ([]gui.Input, error) {
	if a.service.State().Running {
		return nil, a.failure("preflight", errors.New("cannot add files while converting"))
	}
	inputs, err := a.service.Inspect(a.ctx, paths)
	return inputs, a.failure("probe", err)
}

func (a *App) failure(stage string, err error) error {
	if err == nil {
		return nil
	}
	err = diagnostic.Wrap(stage, err)
	problem := diagnostic.Describe(err)
	diagnostic.Emit(a.ctx, diagnostic.Event{Level: "error", Code: problem.Code, Stage: problem.Stage, Message: problem.Detail})
	return diagnostic.Transport(err)
}

func (a *App) ChooseFiles() ([]gui.Input, error) {
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Filters: []wruntime.FileFilter{{DisplayName: "DTS / MKV", Pattern: "*.dts;*.dtshd;*.mkv"}},
	})
	if err != nil {
		return nil, a.failure("dialog", err)
	}
	if len(paths) == 0 {
		// Picker cancellation is an empty addition, not a probe failure.
		// Return [] rather than null so JavaScript can iterate the result.
		return []gui.Input{}, nil
	}
	return a.Inspect(paths)
}

func (a *App) ChooseOutput() (string, error) {
	path, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{})
	return path, a.failure("dialog", err)
}

func (a *App) Start(request gui.Request) (gui.State, error) {
	state, err := a.service.Start(a.ctx, request)
	return state, a.failure("preflight", err)
}
func (a *App) RetryFailed() (gui.State, error) {
	state, err := a.service.RetryFailed(a.ctx)
	return state, a.failure("preflight", err)
}
func (a *App) Logs() (diagnostic.View, error) {
	view, err := a.service.Logs()
	if err != nil {
		err = &diagnostic.Failure{Code: "log_unavailable", Cause: err}
	}
	return view, a.failure("logs", err)
}
func (a *App) State() gui.State          { return a.service.State() }
func (a *App) Cancel()                   { a.service.Cancel() }
func (a *App) BuildInfo() buildinfo.Info { return buildinfo.Current() }

func main() {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	app := &App{}
	directory, logErr := diagnostic.ProgramDirectory()
	var recorder *diagnostic.Recorder
	if logErr == nil {
		recorder, logErr = diagnostic.Open(directory)
	}
	defer func() {
		if recorder != nil {
			if err := recorder.Close(); err != nil {
				log.Printf("Diagnostic logging failed: %v", err)
			}
		}
	}()
	app.service = gui.New(audio.Backend{}, func(state gui.State) {
		if !app.closing.Load() {
			wruntime.EventsEmit(app.ctx, "queue", state)
		}
	})
	app.service.ConfigureDiagnostics(recorder, logErr)
	err = wails.Run(&options.App{
		Title: "HDMA Converter " + buildinfo.Version, Width: 1140, Height: 830, MinWidth: 860, MinHeight: 650,
		BackgroundColour: options.NewRGB(245, 247, 251),
		AssetServer:      &assetserver.Options{Assets: assets},
		DragAndDrop:      &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		OnStartup: func(ctx context.Context) {
			app.ctx, app.stop = context.WithCancel(ctx)
			if recorder != nil {
				app.ctx = diagnostic.WithSink(app.ctx, recorder.Record)
			}
			av.SetLogContext(app.ctx)
			diagnostic.Emit(app.ctx, diagnostic.Event{Level: "info", Code: "application_start", Message: "Version " + buildinfo.Version + "; backend " + audio.BackendVersion + "; " + runtime.GOOS + "/" + runtime.GOARCH})
		},
		OnBeforeClose: func(context.Context) bool {
			// Suppress the worker's terminal queue event before cancelling.
			// Keep the app alive until native cleanup has finished.
			app.closing.Store(true)
			app.stop()
			app.service.Shutdown()
			return false
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
