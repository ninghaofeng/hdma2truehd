//go:build desktop && (darwin || windows)

package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"sync/atomic"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/buildinfo"
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
		return nil, errors.New("cannot add files while converting")
	}
	return a.service.Inspect(a.ctx, paths)
}

func (a *App) ChooseFiles() ([]gui.Input, error) {
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Filters: []wruntime.FileFilter{{DisplayName: "DTS / MKV", Pattern: "*.dts;*.dtshd;*.mkv"}},
	})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		// Picker cancellation is an empty addition, not a probe failure.
		// Return [] rather than null so JavaScript can iterate the result.
		return []gui.Input{}, nil
	}
	return a.Inspect(paths)
}

func (a *App) ChooseOutput() (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{})
}

func (a *App) Start(request gui.Request) (gui.State, error) { return a.service.Start(a.ctx, request) }
func (a *App) State() gui.State                             { return a.service.State() }
func (a *App) Cancel()                                      { a.service.Cancel() }
func (a *App) BuildInfo() buildinfo.Info                    { return buildinfo.Current() }

func main() {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	app := &App{}
	app.service = gui.New(audio.Backend{}, func(state gui.State) {
		if !app.closing.Load() {
			wruntime.EventsEmit(app.ctx, "queue", state)
		}
	})
	err = wails.Run(&options.App{
		Title: "HDMA Converter " + buildinfo.Version, Width: 1140, Height: 830, MinWidth: 860, MinHeight: 650,
		BackgroundColour: options.NewRGB(245, 247, 251),
		AssetServer:      &assetserver.Options{Assets: assets},
		DragAndDrop:      &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		OnStartup:        func(ctx context.Context) { app.ctx, app.stop = context.WithCancel(ctx) },
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
