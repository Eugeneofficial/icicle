//go:build windows && wails

package main

import (
	"embed"
	"fmt"
	"os"

	"icicle/internal/meta"
	"icicle/internal/singleinstance"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend/*
var assets embed.FS

func main() {
	if os.Getenv("ICICLE_ALLOW_MULTI") != "1" {
		ok, err := singleinstance.Acquire("icicle_wails_single_instance_v1")
		if err != nil {
			fmt.Fprintf(os.Stderr, "instance guard failed: %v\n", err)
			os.Exit(1)
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "icicle desktop is already running")
			os.Exit(1)
		}
		defer singleinstance.Release()
	}

	appPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "executable path error: %v\n", err)
		os.Exit(1)
	}

	app := NewApp(appPath)
	err = wails.Run(&options.App{
		Title:                    "icicle " + meta.Version,
		Width:                    1220,
		Height:                   820,
		MinWidth:                 960,
		MinHeight:                640,
		DisableResize:            false,
		Frameless:                false,
		BackgroundColour:         &options.RGBA{R: 12, G: 18, B: 32, A: 1},
		AssetServer:              &assetserver.Options{Assets: assets},
		OnStartup:                app.startup,
		OnShutdown:               app.shutdown,
		Bind:                     []interface{}{app},
		EnableDefaultContextMenu: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "wails run error: %v\n", err)
		os.Exit(1)
	}
}
