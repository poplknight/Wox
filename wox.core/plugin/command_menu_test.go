package plugin

import (
	"testing"

	"wox/common"
)

func TestTriggerCommandMenuReplacesEmptyKeywordResults(t *testing.T) {
	instance := &Instance{Metadata: Metadata{
		TriggerKeywords: []string{"note"},
		Commands: []MetadataCommand{
			{Command: "new", Description: "Create a note"},
			{Command: "search", Description: "Search notes"},
		},
	}}
	response := applyTriggerCommandMenu(instance, Query{Type: QueryTypeInput, TriggerKeyword: "note"}, QueryResponse{
		Results: []QueryResult{{Title: "existing note", IdentityKey: "note:1"}},
	})
	if len(response.Results) != 2 || response.Results[0].Title != "new" || response.Results[1].Title != "search" {
		t.Fatalf("menu = %#v", response.Results)
	}
	if !response.Results[0].Actions[0].PreventHideAfterAction {
		t.Fatal("choosing a command must keep the launcher open")
	}
}

func TestTriggerCommandMenuFiltersAndKeepsUnrelatedContent(t *testing.T) {
	instance := &Instance{Metadata: Metadata{
		Commands: []MetadataCommand{
			{Command: "new", Description: "Create a note"},
			{Command: "search", Description: "Search notes"},
		},
	}}
	filtered := applyTriggerCommandMenu(instance, Query{Type: QueryTypeInput, TriggerKeyword: "note", Search: "sea"}, QueryResponse{
		Results: []QueryResult{{Title: "meeting", IdentityKey: "note:1"}},
	})
	if len(filtered.Results) != 2 || filtered.Results[0].Title != "search" || filtered.Results[1].Title != "meeting" {
		t.Fatalf("filtered menu = %#v", filtered.Results)
	}

	content := applyTriggerCommandMenu(instance, Query{Type: QueryTypeInput, TriggerKeyword: "note", Search: "meeting"}, QueryResponse{
		Results: []QueryResult{{Title: "meeting", IdentityKey: "note:1"}},
	})
	if len(content.Results) != 1 || content.Results[0].Title != "meeting" {
		t.Fatalf("content search = %#v", content.Results)
	}
}

func TestTriggerCommandMenuKeepsPluginOwnedCommands(t *testing.T) {
	instance := &Instance{Metadata: Metadata{
		Commands: []MetadataCommand{{Command: "confetti"}, {Command: "set-volume"}},
	}}
	response := applyTriggerCommandMenu(instance, Query{Type: QueryTypeInput, TriggerKeyword: "xt"}, QueryResponse{
		Results: []QueryResult{{Title: "撒花", IdentityKey: "confetti"}, {Title: "设置音量", IdentityKey: "set-volume"}},
	})
	if len(response.Results) != 2 || response.Results[0].Title != "撒花" {
		t.Fatalf("owned commands were replaced: %#v", response.Results)
	}
}

func TestCommandMenuPlainQueryWaitsForRequiredArgument(t *testing.T) {
	plain := commandMenuPlainQuery("cb", MetadataCommand{
		Command:   "fav",
		QueryHint: &common.QueryHint{Elements: []common.QueryElement{{Id: "search", Kind: common.QueryElementArgument, Required: true}}},
	})
	if plain.QueryText != "cb fav " || plain.QueryHint == nil || plain.QueryHint.Argument("search") != "" {
		t.Fatalf("plain query = %+v", plain)
	}
	ready := commandMenuPlainQuery("note", MetadataCommand{Command: "new"})
	if ready.QueryText != "note new " || ready.QueryHint != nil {
		t.Fatalf("ready query = %+v", ready)
	}
}
