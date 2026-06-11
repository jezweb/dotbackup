package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const Label = "au.dotbackup"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

type plistData struct {
	Label       string
	BinPath     string
	IntervalSec int
	RunAtLoad   bool
	LogPath     string
}

// LogPath is ~/Library/Logs/dotbackup.log — where the scheduled runner's output
// goes so a user (or we) can see what a background backup did.
func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Logs", "dotbackup.log"), nil
}

// The plist carries no secret — just the binary path and the schedule. The binary
// reads the keychain at run time.
var plistTmpl = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>{{.Label}}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{.BinPath}}</string>
    <string>--run-scheduled</string>
  </array>
  <key>StartInterval</key><integer>{{.IntervalSec}}</integer>
  <key>RunAtLoad</key><{{if .RunAtLoad}}true{{else}}false{{end}}/>
  <key>ProcessType</key><string>Standard</string>
  <key>Nice</key><integer>5</integer>
  <key>StandardOutPath</key><string>{{.LogPath}}</string>
  <key>StandardErrorPath</key><string>{{.LogPath}}</string>
</dict>
</plist>
`))

func domain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

// Install writes the LaunchAgent plist and (re)bootstraps it into the user's GUI
// domain so backups run with the app closed.
func Install(binPath string, intervalSec int, runAtLoad bool) error {
	p, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	logPath, err := LogPath()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	if err := plistTmpl.Execute(f, plistData{Label: Label, BinPath: binPath, IntervalSec: intervalSec, RunAtLoad: runAtLoad, LogPath: logPath}); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", domain()+"/"+Label).Run() // ignore "not loaded"
	if out, err := exec.Command("launchctl", "bootstrap", domain(), p).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, string(out))
	}
	return nil
}

// Uninstall boots the agent out and removes the plist.
func Uninstall() error {
	p, err := plistPath()
	if err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", domain()+"/"+Label).Run()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
