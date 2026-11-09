package sys

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
	"wox/util/fuzzymatch"
)

func TestCommandExistsCachesAvailability(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("PATH", directory)
	command := "wox-command-exists-test"
	if runtime.GOOS == "windows" {
		command += ".exe"
	}
	path := filepath.Join(directory, command)
	if err := os.WriteFile(path, nil, 0755); err != nil {
		t.Fatal(err)
	}
	if !commandExists(command) {
		t.Fatal("test command was not found")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if !commandExists(command) {
		t.Fatal("cached command availability was lost")
	}
}

func TestReleasePreparedSearchTextClearsCommandIndex(t *testing.T) {
	plugin := &SysPlugin{
		commandSearchIndexes: map[i18n.LangCode][][]*fuzzymatch.PreparedText{
			"en_US": {{fuzzymatch.PrepareText("lock")}},
		},
	}
	plugin.ReleasePreparedSearchText()
	if plugin.commandSearchIndexes != nil {
		t.Fatalf("released command index has %d languages, want none", len(plugin.commandSearchIndexes))
	}
}

func TestDevCommandsIncludeToolbarProgressPreview(t *testing.T) {
	for _, command := range (&SysPlugin{}).buildDevCommands() {
		if command.ID == "test_toolbar_progress" {
			if command.Action == nil || !command.PreventHideAfterAction {
				t.Fatal("toolbar progress preview must stay visible and executable")
			}
			return
		}
	}

	t.Fatal("toolbar progress preview command is missing")
}

func TestDevCommandsIncludeOpenOnboarding(t *testing.T) {
	for _, command := range (&SysPlugin{}).buildDevCommands() {
		if command.ID == "open_onboarding" {
			if command.Action == nil || !command.PreventHideAfterAction {
				t.Fatal("open onboarding must stay visible and executable")
			}
			return
		}
	}

	t.Fatal("open onboarding command is missing")
}

func TestCommandsIncludeConfetti(t *testing.T) {
	for _, command := range (&SysPlugin{}).buildCommands() {
		if command.ID == "confetti" {
			if command.Action == nil {
				t.Fatal("confetti command must be executable")
			}
			return
		}
	}

	t.Fatal("confetti command is missing")
}

func TestLimitSysQueryResultsKeepsHighestScoresStable(t *testing.T) {
	results := []plugin.QueryResult{{Title: "first", Score: 100}, {Title: "second", Score: 100}}
	for score := int64(31); score >= 0; score-- {
		results = append(results, plugin.QueryResult{Title: fmt.Sprintf("score-%d", score), Score: score})
	}

	limited := limitSysQueryResults(results)
	if len(limited) != sysQueryResultLimit {
		t.Fatalf("result count = %d, want %d", len(limited), sysQueryResultLimit)
	}
	if limited[0].Title != "first" || limited[1].Title != "second" || limited[len(limited)-1].Score != 4 {
		t.Fatalf("limited results did not preserve the highest scores and stable ties: %#v", limited)
	}
}

func TestSetVolumeCommandAcceptsInlinePercentage(t *testing.T) {
	percent, valid := parseVolumePercent("set volume 30")
	if !valid || percent != 30 {
		t.Fatalf("parsed volume = %d, %t, want 30, true", percent, valid)
	}

	plugin := &SysPlugin{}
	matched, _ := plugin.commandMatches(SysCommand{ID: "set-volume"}, "set volume 30", nil, nil, false)
	fixedPresetMatched, _ := plugin.commandMatches(SysCommand{ID: "set-volume-25"}, "set volume 25", nil, nil, false)
	bareNumberMatched, _ := plugin.commandMatches(SysCommand{ID: "set-volume"}, "30", nil, nil, false)
	if !matched || fixedPresetMatched || bareNumberMatched {
		t.Fatalf("matches = %t, preset matches = %t, bare number matches = %t, want true, false, false", matched, fixedPresetMatched, bareNumberMatched)
	}

	command := SysCommand{ID: "set-volume", Title: "Set Volume", QueryHint: &common.QueryHint{Elements: []common.QueryElement{{Id: "volume", Kind: common.QueryElementArgument, Required: true}}}}
	incompleteAction := plugin.buildCommandAction(pluginQuery(), command, nil)
	completeAction := plugin.buildCommandAction(pluginQuery(), command, map[string]string{sysCommandVolumeContextKey: "30"})
	if !incompleteAction.PreventHideAfterAction || incompleteAction.Action == nil || completeAction.PreventHideAfterAction {
		t.Fatal("incomplete volume commands must continue query input; complete commands must execute normally")
	}
}

func TestStructuredVolumeUsesArgumentAndValidatesRange(t *testing.T) {
	for _, value := range []string{"", "-1", "101", "1.5", "abc", "0", "50", "100", "50%"} {
		structure := &common.QueryHint{Elements: []common.QueryElement{{Id: "volume", Kind: common.QueryElementArgument, Value: value}}}
		query := plugin.Query{Search: "set volume 75", QueryHint: structure}
		got := buildSetVolumeContextData(query)
		_, valid := parseVolumePercent(value)
		if valid != (got[sysCommandVolumeContextKey] != "") {
			t.Fatalf("value %q context = %v", value, got)
		}
	}
}

func pluginQuery() plugin.Query {
	return plugin.Query{}
}

func TestTriggerKeywordContinuesVolumeCommand(t *testing.T) {
	if commandArgumentQueryText("xt", "set-volume") != "xt set-volume " {
		t.Fatalf("continuation query = %q", commandArgumentQueryText("xt", "set-volume"))
	}

	dataDir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, dataDir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(dataDir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatal(err)
	}

	executed := ""
	volumeHint := &common.QueryHint{Elements: []common.QueryElement{{Id: "volume", Kind: common.QueryElementArgument, Placeholder: "0-100", Required: true}}}
	sysPlugin := &SysPlugin{commands: []SysCommand{
		{ID: "confetti", Title: "撒花", Action: func(context.Context, plugin.ActionContext) { executed = "confetti" }},
		{ID: "set-volume", Title: "设置音量", QueryHint: volumeHint, BuildContextData: buildSetVolumeContextData, Action: func(context.Context, plugin.ActionContext) { executed = "volume" }},
	}}

	list := sysPlugin.Query(context.Background(), plugin.Query{TriggerKeyword: "xt"})
	if len(list.Results) != 2 {
		t.Fatalf("candidate count = %d, want confetti and set volume", len(list.Results))
	}
	var volumeResult, confettiResult plugin.QueryResult
	for _, result := range list.Results {
		switch result.Title {
		case "设置音量":
			volumeResult = result
		case "撒花":
			confettiResult = result
		}
	}
	if len(volumeResult.Actions) != 1 || !volumeResult.Actions[0].PreventHideAfterAction {
		t.Fatalf("set volume from the candidate list must continue input: %#v", volumeResult.Actions)
	}
	confettiResult.Actions[0].Action(context.Background(), plugin.ActionContext{})
	if executed != "confetti" {
		t.Fatal("confetti should execute from the candidate list")
	}

	waiting := sysPlugin.Query(context.Background(), plugin.Query{TriggerKeyword: "xt", Command: "set-volume"})
	if len(waiting.Results) != 1 || len(waiting.Results[0].Actions) != 0 {
		t.Fatalf("waiting volume result = %#v", waiting.Results)
	}

	ready := sysPlugin.Query(context.Background(), plugin.Query{TriggerKeyword: "xt", Command: "set-volume", Search: "10"})
	if len(ready.Results) != 1 || len(ready.Results[0].Actions) != 1 {
		t.Fatalf("ready volume result = %#v", ready.Results)
	}
	ready.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
	if executed != "volume" {
		t.Fatal("a completed volume command should execute")
	}
}

func TestCommandSuggestionHintKeepsSystemCommands(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, dataDir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(dataDir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatal(err)
	}

	p := &SysPlugin{commands: []SysCommand{
		{ID: "set-volume", Title: "设置音量", Aliases: []string{"set volume", "设置音量"}},
		{ID: "lock", Title: "锁定", Aliases: []string{"lock", "锁定"}, Action: func(context.Context, plugin.ActionContext) {}},
	}}
	suggestion := &common.QueryHint{CommandSuggestions: true, Elements: []common.QueryElement{
		{Id: "command", Kind: common.QueryElementText, Text: "xt "},
		{Id: "command-suggestion", Kind: common.QueryElementArgument, Suggestions: []string{"set-volume", "lock"}},
	}}

	response := p.Query(context.Background(), plugin.Query{TriggerKeyword: "xt", QueryHint: suggestion})
	if len(response.Results) != 2 {
		t.Fatalf("result count = %d, want lock and set volume", len(response.Results))
	}

	filtered := p.Query(context.Background(), plugin.Query{TriggerKeyword: "xt", Search: "lock", QueryHint: suggestion})
	if len(filtered.Results) != 1 || filtered.Results[0].Title != "锁定" {
		t.Fatalf("filtered results = %#v, want lock", filtered.Results)
	}

	inline := buildSetVolumeContextData(plugin.Query{Search: "set volume 40", QueryHint: suggestion})
	if inline[sysCommandVolumeContextKey] != "40" {
		t.Fatalf("inline volume = %q, want 40", inline[sysCommandVolumeContextKey])
	}
}

func TestStructuredVolumeOnlyExecutesValidAction(t *testing.T) {
	executed := false
	p := &SysPlugin{commands: []SysCommand{{ID: "set-volume", Title: "Volume", BuildContextData: buildSetVolumeContextData, Action: func(context.Context, plugin.ActionContext) { executed = true }}}}
	for _, value := range []string{"", "101", "50"} {
		q := plugin.Query{QueryHint: &common.QueryHint{Elements: []common.QueryElement{{Id: "volume", Kind: "argument", Value: value}}}}
		response := p.Query(context.Background(), q)
		if executed {
			t.Fatal("query executed a volume action")
		}
		if len(response.Results) != 1 {
			t.Fatal("expected volume result only")
		}
		if value != "50" && len(response.Results[0].Actions) != 0 {
			t.Fatal("invalid value exposes an action")
		}
		if value == "50" {
			response.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
			if !executed {
				t.Fatal("valid action missing")
			}
		}
	}
}

// TestRestartWoxCommandIsDiscoverable keeps app restart distinct from computer reboot.
func TestRestartWoxCommandIsDiscoverable(t *testing.T) {
	plugin := &SysPlugin{}
	for _, command := range plugin.getMetadataCommands() {
		if command.Command == "restart_wox" {
			for _, alias := range command.Aliases {
				if alias == "restart wox" {
					return
				}
			}
			t.Fatal("restart Wox search alias is missing")
		}
	}
	t.Fatal("restart Wox command is missing")
}
