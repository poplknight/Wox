package mediaplayer

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"wox/plugin"
)

func init() {
	plugin.RegisterMediaRuntime(platformMediaRuntime{})
}

// platformMediaRuntime is the settings-facing view of the platform now-playing component.
type platformMediaRuntime struct{}

func (platformMediaRuntime) NeedsInstall() bool { return mediaRuntimeNeedsInstall() }

func (platformMediaRuntime) Installed() bool { return mediaRuntimeInstalled() }

func (platformMediaRuntime) Install(ctx context.Context) error {
	return installMediaRuntime(ctx)
}

var mediaRuntimeInstallMu sync.Mutex

// installMediaRuntime installs the platform component once, even if query and settings both ask.
func installMediaRuntime(ctx context.Context) error {
	mediaRuntimeInstallMu.Lock()
	defer mediaRuntimeInstallMu.Unlock()
	return installPlatformMediaRuntime(ctx)
}

// queryNeedsRuntimeInstallNotice reports a media query that should offer installation.
func queryNeedsRuntimeInstallNotice(query plugin.Query, needsInstall, installed bool) bool {
	if !needsInstall || installed {
		return false
	}
	if !query.IsGlobalQuery() {
		return true
	}
	text := strings.TrimSpace(query.Search)
	if text == "" {
		text = strings.TrimSpace(query.RawQuery)
	}
	return strings.EqualFold(text, "media")
}

var mediaComponentFileNames = []string{"adapter.pl", "WoxMR.pm", "woxmr.bundle"}

// mediaComponentFilesReady reports whether a directory contains the macOS now-playing adapter.
func mediaComponentFilesReady(dir string) bool {
	for _, name := range mediaComponentFileNames {
		if !fileExists(filepath.Join(dir, name)) {
			return false
		}
	}
	return true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// copyMediaComponentFiles copies the macOS adapter into a directory startup will not delete.
func copyMediaComponentFiles(from, to string) error {
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, name := range mediaComponentFileNames {
		if err := copyFile(filepath.Join(from, name), filepath.Join(to, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = io.Copy(target, source)
	return err
}

var bundledMediaRuntimeMirrored atomic.Bool

// mirrorBundledMediaRuntime keeps an extracted adapter outside the directory startup replaces.
func mirrorBundledMediaRuntime(bundled, durable string) {
	if !bundledMediaRuntimeMirrored.CompareAndSwap(false, true) {
		return
	}
	if mediaComponentFilesReady(durable) || !mediaComponentFilesReady(bundled) {
		return
	}
	if err := copyMediaComponentFiles(bundled, durable); err != nil {
		bundledMediaRuntimeMirrored.Store(false)
	}
}
