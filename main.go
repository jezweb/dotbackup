// Command dotbackup is the desktop app. The same binary also runs two headless
// modes — the one launchd schedules, and the one restic's RESTIC_PASSWORD_COMMAND
// invokes — dispatched before any UI starts.
package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/jezweb/dotbackup/internal/schedule"
	"github.com/jezweb/dotbackup/internal/secret"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--print-passphrase":
			// restic invokes this via RESTIC_PASSWORD_COMMAND; DOTBACKUP_USER is
			// set in the runner env, so the passphrase never sits in argv.
			pass, err := secret.ReadPassphrase(os.Getenv("DOTBACKUP_USER"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Print(pass)
			return
		case "--run-scheduled":
			self, _ := os.Executable()
			if err := schedule.RunScheduled(context.Background(), self+" --print-passphrase"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}

	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "dotbackup",
		Width:            760,
		Height:           600,
		MinWidth:         560,
		MinHeight:        440,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "dotbackup",
				Message: "Encrypted backup to your own Cloudflare R2.\n© 2026 Jezweb Pty Ltd",
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
