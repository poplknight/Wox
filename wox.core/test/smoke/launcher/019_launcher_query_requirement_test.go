//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

const (
	queryRequirementTrigger    = "qreqsm"
	queryRequirementPluginID   = "com.wox.smoke.queryrequirement"
	queryRequirementAccessKey  = "smoke-access-key"
	queryRequirementReadyTitle = "query requirement ready:" + queryRequirementAccessKey
	queryRequirementPluginFile = "Wox.Plugin.SmokeQueryRequirement.py"
)

// Test019LauncherQueryRequirement verifies a plugin-scoped query stays on the core setup form until the required setting is saved.
// Flow: install a single-file plugin that requires accessKey -> query its trigger -> fill the requirement field -> save.
// Evidence: the blocked generation shows only the requirement form, then the refreshed generation shows the plugin result that reads the persisted key.
func Test019LauncherQueryRequirement(t *testing.T) {
	writeQueryRequirementPlugin(t, queryRequirementPluginFile, queryRequirementAnyQueryPluginSource(queryRequirementPluginID, "Query Requirement Smoke", queryRequirementTrigger))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		blocked := waitForQueryRequirementForm(t, ctx, client, queryRequirementTrigger+" ")
		smoke.AssertNoDiagnostics(t, blocked)
		if smoke.HasLauncherResultLabel(blocked, queryRequirementReadyTitle) {
			t.Fatalf("blocked query already exposed the plugin result %q", queryRequirementReadyTitle)
		}
		fillAndSaveQueryRequirementField(t, ctx, client, queryRequirementAccessKey)
		ready := waitForQueryRequirementRefresh(t, ctx, client, queryRequirementReadyTitle)
		smoke.AssertNoDiagnostics(t, ready)
	})
}
