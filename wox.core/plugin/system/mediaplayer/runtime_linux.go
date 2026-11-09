package mediaplayer

import "context"

// Linux reads MPRIS over the session bus, so there is no separate component to install.
func mediaRuntimeNeedsInstall() bool { return false }

func mediaRuntimeInstalled() bool { return true }

func installPlatformMediaRuntime(context.Context) error { return nil }
