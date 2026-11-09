//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

const (
	queryRequirementCommandTrigger    = "qreqcmd"
	queryRequirementCommandName       = "download"
	queryRequirementCommandPluginID   = "com.wox.smoke.queryrequirement.command"
	queryRequirementCommandPath       = "smoke-download-path"
	queryRequirementCommandPluginFile = "Wox.Plugin.SmokeQueryRequirementCommand.py"
	queryRequirementCommandOpenTitle  = "query requirement command::"
	queryRequirementCommandReadyTitle = "query requirement command:" + queryRequirementCommandName + ":" + queryRequirementCommandPath
)

// Test021LauncherQueryRequirementCommandScope verifies QueryWithCommand blocks only that command while a no-command query still reaches the plugin.
// Flow: install a plugin that requires downloadPath for download -> query the trigger without a command -> query the download command -> fill and save.
// Evidence: the no-command generation shows the plugin result, the command generation shows only the setup form, then the refreshed command generation reads the persisted path.
func Test021LauncherQueryRequirementCommandScope(t *testing.T) {
	writeQueryRequirementPlugin(t, queryRequirementCommandPluginFile, queryRequirementCommandPluginSource(queryRequirementCommandPluginID, "Query Requirement Command Smoke", queryRequirementCommandTrigger, queryRequirementCommandName))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		openQuery := queryRequirementCommandTrigger + " "
		opened := waitForQueryRequirementResult(t, ctx, client, openQuery, queryRequirementCommandOpenTitle)
		smoke.AssertNoDiagnostics(t, opened)
		if queryRequirementFormVisible(opened) {
			t.Fatal("no-command query showed the query requirement form")
		}

		commandQuery := queryRequirementCommandTrigger + " " + queryRequirementCommandName + " "
		blocked := waitForQueryRequirementForm(t, ctx, client, commandQuery)
		smoke.AssertNoDiagnostics(t, blocked)
		if smoke.HasLauncherResultLabel(blocked, queryRequirementCommandOpenTitle) || smoke.HasLauncherResultLabel(blocked, queryRequirementCommandReadyTitle) {
			t.Fatal("command query exposed a plugin result before downloadPath was saved")
		}

		fillAndSaveQueryRequirementField(t, ctx, client, queryRequirementCommandPath)
		ready := waitForQueryRequirementRefresh(t, ctx, client, queryRequirementCommandReadyTitle)
		smoke.AssertNoDiagnostics(t, ready)
	})
}
