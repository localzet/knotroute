//go:build windows

package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/localzet/knotroute/internal/clientruntime"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var desktopAssets embed.FS

func runDesktop() {
	dataDir := defaultDataDir()
	runtime, err := clientruntime.Open(dataDir)
	if err != nil {
		panic(fmt.Errorf("open KnotRoute runtime: %w", err))
	}
	app := newDesktopApp(runtime, dataDir)
	err = wails.Run(&options.App{
		Title:             "KnotRoute",
		Width:             1360,
		Height:            860,
		MinWidth:          1040,
		MinHeight:         680,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		AssetServer:       &assetserver.Options{Assets: desktopAssets},
		BackgroundColour:  &options.RGBA{R: 8, G: 13, B: 19, A: 255},
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnShutdown:        app.shutdown,
		Bind:              []interface{}{app},
		ErrorFormatter:    func(err error) any { return err.Error() },
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "4dcbb5db-05c8-40a4-b2fd-24bcd490d5b7",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				ctx := app.context()
				wailsruntime.WindowUnminimise(ctx)
				wailsruntime.Show(ctx)
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			Theme:                windows.Dark,
			WebviewUserDataPath:  filepath.Join(dataDir, "webview2"),
		},
	})
	if err != nil {
		panic(err)
	}
}

func defaultDataDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "KnotRoute")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "KnotRoute")
	}
	return filepath.Join(dir, "KnotRoute")
}
