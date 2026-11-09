package mediaplayer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"wox/util"
)

func mediaRuntimeNeedsInstall() bool { return true }

func durableMediaRuntimeDir() string {
	return filepath.Join(util.GetLocation().GetWoxDataDirectory(), "woxmr")
}

func bundledMediaRuntimeDir() string {
	return filepath.Join(util.GetLocation().GetOthersDirectory(), "woxmr")
}

// mediaRuntimeInstalled reports whether the Perl MediaRemote adapter can be loaded.
// Startup replaces ~/.wox/others, so a bundled copy is mirrored into ~/.wox/woxmr.
func mediaRuntimeInstalled() bool {
	durable := durableMediaRuntimeDir()
	if mediaComponentFilesReady(durable) {
		return true
	}
	bundled := bundledMediaRuntimeDir()
	if !mediaComponentFilesReady(bundled) {
		return false
	}
	mirrorBundledMediaRuntime(bundled, durable)
	return true
}

// mediaRuntimeScriptPath returns the adapter that should run for the current install.
func mediaRuntimeScriptPath() string {
	if mediaComponentFilesReady(durableMediaRuntimeDir()) {
		return filepath.Join(durableMediaRuntimeDir(), "adapter.pl")
	}
	return filepath.Join(bundledMediaRuntimeDir(), "adapter.pl")
}

func installPlatformMediaRuntime(ctx context.Context) error {
	durable := durableMediaRuntimeDir()
	if mediaComponentFilesReady(durable) {
		return nil
	}
	bundled := bundledMediaRuntimeDir()
	if mediaComponentFilesReady(bundled) {
		return copyMediaComponentFiles(bundled, durable)
	}
	source, err := buildDarwinMediaRuntime(ctx)
	if err != nil {
		return err
	}
	return copyMediaComponentFiles(source, durable)
}

// buildDarwinMediaRuntime compiles WoxMR from the checkout that produced this binary.
func buildDarwinMediaRuntime(ctx context.Context) (string, error) {
	repo := findWoxRepoRoot()
	if repo == "" {
		return "", fmt.Errorf("media component source was not found")
	}
	coreDir := filepath.Join(repo, "wox.core")
	built := filepath.Join(coreDir, "resource", "others", "woxmr")
	if !mediaComponentFilesReady(built) {
		cmd := exec.CommandContext(ctx, "make", "-C", coreDir, "woxmr-build")
		output, err := cmd.CombinedOutput()
		if err != nil {
			text := trimCommandOutput(output)
			if text == "" {
				return "", fmt.Errorf("build media component: %w", err)
			}
			return "", fmt.Errorf("build media component: %w: %s", err, text)
		}
	}
	if !mediaComponentFilesReady(built) {
		return "", fmt.Errorf("media component build did not produce the adapter")
	}
	return built, nil
}

func trimCommandOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	const limit = 500
	if len(text) <= limit {
		return text
	}
	return text[len(text)-limit:]
}

// findWoxRepoRoot walks from the binary and working directory to the Wox checkout.
func findWoxRepoRoot() string {
	candidates := make([]string, 0, 2)
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	for _, start := range candidates {
		dir := start
		for i := 0; i < 8; i++ {
			marker := filepath.Join(dir, "wox.core", "plugin", "system", "mediaplayer", "woxmr", "Makefile.PL")
			if fileExists(marker) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}
