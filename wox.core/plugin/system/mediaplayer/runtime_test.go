package mediaplayer

import (
	"os"
	"path/filepath"
	"testing"

	"wox/plugin"
)

func TestMediaComponentFilesReady(t *testing.T) {
	dir := t.TempDir()
	if mediaComponentFilesReady(dir) {
		t.Fatal("empty directory reported a media component")
	}
	for _, name := range mediaComponentFileNames {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !mediaComponentFilesReady(dir) {
		t.Fatal("complete directory was not ready")
	}
}

func TestCopyMediaComponentFiles(t *testing.T) {
	from := t.TempDir()
	to := filepath.Join(t.TempDir(), "woxmr")
	for _, name := range mediaComponentFileNames {
		if err := os.WriteFile(filepath.Join(from, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyMediaComponentFiles(from, to); err != nil {
		t.Fatal(err)
	}
	if !mediaComponentFilesReady(to) {
		t.Fatal("copied directory was not ready")
	}
}

func TestQueryNeedsRuntimeInstallNotice(t *testing.T) {
	mediaQuery := plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "media"}
	if !queryNeedsRuntimeInstallNotice(mediaQuery, true, false) {
		t.Fatal("media keyword should offer installation")
	}
	if queryNeedsRuntimeInstallNotice(mediaQuery, true, true) {
		t.Fatal("installed component should not offer installation")
	}
	if queryNeedsRuntimeInstallNotice(mediaQuery, false, false) {
		t.Fatal("platforms without a component should not offer installation")
	}

	globalMedia := plugin.Query{Type: plugin.QueryTypeInput, Search: "media"}
	if !queryNeedsRuntimeInstallNotice(globalMedia, true, false) {
		t.Fatal("global media query should offer installation")
	}
	play := plugin.Query{Type: plugin.QueryTypeInput, Search: "play"}
	if queryNeedsRuntimeInstallNotice(play, true, false) {
		t.Fatal("playback commands stay on their own results")
	}
}
