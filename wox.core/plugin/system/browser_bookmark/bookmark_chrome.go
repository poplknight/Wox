package browserbookmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"wox/plugin"
	"wox/plugin/system"
	"wox/util"
	"wox/util/browser"

	"github.com/mitchellh/go-homedir"
)

var chromeBookmarkProfiles = []string{"Default", "Profile 1", "Profile 2", "Profile 3"}

// chromiumBookmarkFileNames are the plaintext JSON files Chromium browsers write in a profile.
// Chrome 146+ with account bookmark sync stores the real bookmarks in AccountBookmarks and
// leaves Bookmarks as an empty skeleton. EncryptedBookmarks is OS-encrypted and is not readable here.
var chromiumBookmarkFileNames = []string{"Bookmarks", "AccountBookmarks"}

func init() {
	registerBrowserBookmarkLoader(browser.BrowserIDChrome, func(c *BrowserBookmarkPlugin, ctx context.Context) []Bookmark {
		var bookmarks []Bookmark
		for _, profile := range chromeBookmarkProfiles {
			bookmarks = append(bookmarks, c.loadChromeBookmark(ctx, profile)...)
		}
		return bookmarks
	})
}

func (c *BrowserBookmarkPlugin) loadChromeBookmark(ctx context.Context, profile string) []Bookmark {
	switch {
	case util.IsMacOS():
		return c.loadChromeBookmarkInMacos(ctx, profile)
	case util.IsWindows():
		return c.loadChromeBookmarkInWindows(ctx, profile)
	case util.IsLinux():
		return c.loadChromeBookmarkInLinux(ctx, profile)
	default:
		return []Bookmark{}
	}
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInMacos(ctx context.Context, profile string) []Bookmark {
	return c.loadChromiumBookmarkFiles(ctx, fmt.Sprintf("~/Library/Application Support/Google/Chrome/%s", profile), "/", "Chrome")
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInWindows(ctx context.Context, profile string) []Bookmark {
	// Use a different approach to avoid fmt.Sprintf converting %% to %
	profileDir := "%%LOCALAPPDATA%%\\Google\\Chrome\\User Data\\" + profile
	return c.loadChromiumBookmarkFiles(ctx, profileDir, "\\", "Chrome")
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInLinux(ctx context.Context, profile string) []Bookmark {
	return c.loadChromiumBookmarkFiles(ctx, fmt.Sprintf("~/.config/google-chrome/%s", profile), "/", "Chrome")
}

// loadChromiumBookmarkFiles reads plaintext Chromium bookmark JSON files from a profile directory.
func (c *BrowserBookmarkPlugin) loadChromiumBookmarkFiles(ctx context.Context, profileDir string, separator string, browserName string) []Bookmark {
	var bookmarks []Bookmark
	for _, fileName := range chromiumBookmarkFileNames {
		bookmarks = append(bookmarks, c.loadBookmarkFromFile(ctx, profileDir+separator+fileName, browserName)...)
	}
	return bookmarks
}

func (c *BrowserBookmarkPlugin) loadBookmarkFromFile(ctx context.Context, bookmarkPath string, browserName string) []Bookmark {
	var bookmarkLocation string
	var err error

	if strings.Contains(bookmarkPath, "%%LOCALAPPDATA%%") {
		// Windows path with environment variable
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return []Bookmark{}
		}
		bookmarkLocation = strings.Replace(bookmarkPath, "%%LOCALAPPDATA%%", localAppData, 1)
	} else {
		// Unix-style path
		bookmarkLocation, err = homedir.Expand(bookmarkPath)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error expanding %s bookmark path: %s", browserName, err.Error()))
			return []Bookmark{}
		}
	}

	if _, err := os.Stat(bookmarkLocation); os.IsNotExist(err) {
		return []Bookmark{}
	}

	file, readErr := os.ReadFile(bookmarkLocation)
	if readErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error reading %s bookmark file: %s", browserName, readErr.Error()))
		return []Bookmark{}
	}

	results, parseErr := parseChromiumBookmarkFile(file)
	if parseErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error parsing %s bookmark file: %s", browserName, parseErr.Error()))
		return []Bookmark{}
	}
	for index := range results {
		icon := browserBookmarkIcon
		if cachedIcon, ok := system.GetWebsiteIconFromCacheOnly(ctx, results[index].Url); ok {
			icon = cachedIcon
		}
		results[index].Icon = icon
	}
	return results
}

type chromiumBookmarkNode struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	URL      string                 `json:"url"`
	Children []chromiumBookmarkNode `json:"children"`
}

type chromiumBookmarkFile struct {
	Roots map[string]chromiumBookmarkNode `json:"roots"`
}

// parseChromiumBookmarkFile walks the bookmark tree. Chrome writes a folder's name after its
// children, so a regex that skips ahead to the next "type": "url" attaches that folder name to
// the following bookmark and drops the real title.
func parseChromiumBookmarkFile(file []byte) ([]Bookmark, error) {
	var parsed chromiumBookmarkFile
	if err := json.Unmarshal(file, &parsed); err != nil {
		return nil, err
	}
	var results []Bookmark
	var walk func(node chromiumBookmarkNode)
	walk = func(node chromiumBookmarkNode) {
		if node.Type == "url" && strings.TrimSpace(node.URL) != "" {
			results = append(results, Bookmark{Name: strings.Clone(node.Name), Url: strings.Clone(node.URL)})
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range parsed.Roots {
		walk(root)
	}
	return results, nil
}
