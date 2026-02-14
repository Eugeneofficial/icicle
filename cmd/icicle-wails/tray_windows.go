//go:build windows && wails

package main

import (
	_ "embed"
	"sync"

	"github.com/getlantern/systray"
)

//go:embed tray.ico
var trayIcon []byte

type trayBridge struct {
	quit chan struct{}
	once sync.Once
}

func startTray(openWindow func()) *trayBridge {
	t := &trayBridge{quit: make(chan struct{})}
	go func() {
		systray.Run(func() {
			if len(trayIcon) > 0 {
				systray.SetIcon(trayIcon)
			}
			systray.SetTooltip("icicle")
			systray.SetTitle("icicle")
			openItem := systray.AddMenuItem("Open icicle", "Show window")
			systray.AddSeparator()
			exitItem := systray.AddMenuItem("Exit", "Quit app")
			go func() {
				for {
					select {
					case <-openItem.ClickedCh:
						openWindow()
					case <-exitItem.ClickedCh:
						t.Close()
						return
					case <-t.quit:
						return
					}
				}
			}()
		}, func() {})
	}()
	return t
}

func (t *trayBridge) Close() {
	t.once.Do(func() {
		close(t.quit)
		systray.Quit()
	})
}
