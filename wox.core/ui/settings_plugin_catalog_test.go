package ui

import (
	"testing"

	"wox/plugin"
)

func TestInstalledCatalogSkipsHiddenSystemPlugins(t *testing.T) {
	if pluginListedInInstalledCatalog(&plugin.Instance{Metadata: plugin.Metadata{HiddenFromPluginList: true}}) {
		t.Fatal("hidden built-in plugin should stay out of the installed catalog")
	}
	if !pluginListedInInstalledCatalog(&plugin.Instance{Metadata: plugin.Metadata{Name: "Clipboard"}}) {
		t.Fatal("ordinary plugin should stay in the installed catalog")
	}
	if pluginListedInInstalledCatalog(nil) {
		t.Fatal("nil plugin should stay out of the installed catalog")
	}
}
