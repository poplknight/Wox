package plugin

import (
	"context"
	"errors"
)

// ErrMediaRuntimeUnavailable means the operating system has no now-playing component to install.
var ErrMediaRuntimeUnavailable = errors.New("media runtime unavailable")

// MediaRuntime reports and installs the platform component that reads now-playing info.
type MediaRuntime interface {
	NeedsInstall() bool
	Installed() bool
	Install(ctx context.Context) error
}

var registeredMediaRuntime MediaRuntime

// RegisterMediaRuntime publishes the media player's platform component for settings and queries.
func RegisterMediaRuntime(runtime MediaRuntime) {
	registeredMediaRuntime = runtime
}

// CurrentMediaRuntime returns the registered now-playing component when the media plugin is loaded.
func CurrentMediaRuntime() MediaRuntime {
	return registeredMediaRuntime
}
