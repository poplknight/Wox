package plugin

import (
	"context"
	"strings"

	"wox/common"
	"wox/common/icons"
)

// applyTriggerCommandMenu shows a plugin's subcommands as results when the user has
// entered its trigger keyword and has not chosen a command yet.
func applyTriggerCommandMenu(instance *Instance, query Query, response QueryResponse) QueryResponse {
	if instance == nil || query.IsGlobalQuery() || strings.TrimSpace(query.Command) != "" {
		return response
	}
	commands := instance.GetQueryCommands()
	if len(commands) == 0 {
		return response
	}
	search := strings.TrimSpace(query.Search)
	if resultsCoverCommands(response.Results, commands, search) {
		return response
	}

	menu := make([]QueryResult, 0, len(commands))
	for _, command := range commands {
		if !commandMatchesMenuSearch(command, search) || resultCoversCommand(response.Results, command.Command) {
			continue
		}
		menu = append(menu, newCommandMenuResult(instance, query.TriggerKeyword, command, len(menu)))
	}
	if len(menu) == 0 {
		return response
	}
	// Search-first plugins keep their own rows and gain the command choices above them.
	if instance.Metadata.IsSupportFeature(MetadataFeatureDisableAutoCommandHint) || search != "" {
		response.Results = append(menu, response.Results...)
		return response
	}
	response.Results = menu
	return response
}

func commandMatchesMenuSearch(command MetadataCommand, search string) bool {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return true
	}
	if strings.Contains(strings.ToLower(command.Command), search) {
		return true
	}
	return strings.Contains(strings.ToLower(string(command.Description)), search)
}

func resultsCoverCommands(results []QueryResult, commands []MetadataCommand, search string) bool {
	matched := 0
	covered := 0
	for _, command := range commands {
		if !commandMatchesMenuSearch(command, search) {
			continue
		}
		matched++
		if resultCoversCommand(results, command.Command) {
			covered++
		}
	}
	return matched > 0 && covered == matched
}

func resultCoversCommand(results []QueryResult, command string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	for _, result := range results {
		identity := strings.ToLower(strings.TrimSpace(result.IdentityKey))
		title := strings.ToLower(strings.TrimSpace(result.Title))
		if identity == command || title == command || strings.HasSuffix(identity, ":"+command) {
			return true
		}
	}
	return false
}

func newCommandMenuResult(instance *Instance, triggerKeyword string, command MetadataCommand, index int) QueryResult {
	trigger := strings.TrimSpace(triggerKeyword)
	commandName := command.Command
	return QueryResult{
		Title:       commandName,
		SubTitle:    string(command.Description),
		IdentityKey: "command-menu:" + commandName,
		Score:       int64(100000 - index),
		Icon:        instance.Metadata.GetIconOrDefault(instance.PluginDirectory, icons.Get(icons.ActionOpen)),
		Actions: []QueryResultAction{{
			Name:                   commandName,
			Icon:                   icons.Get(icons.ActionOpen),
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, _ ActionContext) {
				ui := GetPluginManager().GetUI()
				if ui == nil {
					return
				}
				plain := commandMenuPlainQuery(trigger, command)
				ui.ChangeQuery(ctx, plain)
			},
		}},
	}
}

// commandMenuPlainQuery moves the input to the selected subcommand and keeps any required argument hint.
func commandMenuPlainQuery(triggerKeyword string, command MetadataCommand) common.PlainQuery {
	text := strings.TrimSpace(triggerKeyword) + " " + command.Command + " "
	plain := common.PlainQuery{QueryType: QueryTypeInput, QueryText: text}
	if command.QueryHint == nil {
		return plain
	}
	for _, element := range command.QueryHint.Elements {
		if !element.Required {
			continue
		}
		structure := command.QueryHint.Clone()
		structure.Elements = append([]common.QueryElement{{Id: "command", Kind: common.QueryElementText, Text: text}}, structure.Elements...)
		plain.QueryText = structure.PlainText()
		plain.QueryHint = structure
		return plain
	}
	return plain
}
