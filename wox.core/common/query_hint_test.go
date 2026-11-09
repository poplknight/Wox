package common

import (
	"encoding/json"
	"testing"
)

func TestQueryHintRoundTripAndIsolation(t *testing.T) {
	s := &QueryHint{Elements: []QueryElement{{Id: "command", Kind: QueryElementText, Text: "gh issues "}, {Id: "repo", Kind: QueryElementArgument, Placeholder: "Repository"}, {Id: "issue", Kind: QueryElementBlock, Value: "owner/repo#6"}}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if s.PlainText() != "gh issues owner/repo#6" {
		t.Fatal(s.PlainText())
	}
	clone := s.Clone()
	clone.Elements[1].Value = "changed"
	if s.Argument("repo") != "" {
		t.Fatal("clone mutated the template")
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored QueryHint
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.PlainText() != s.PlainText() {
		t.Fatal("round trip lost content")
	}
	clone.Elements[1].Id = "command"
	if clone.Validate() == nil {
		t.Fatal("duplicate IDs accepted")
	}
	clone.Elements[1].Id = "repo"
	clone.Elements[1].Kind = "unknown"
	if clone.Validate() == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestQueryHintCannotReplaceQueryContent(t *testing.T) {
	hint := &QueryHint{Elements: []QueryElement{{Id: "command", Kind: QueryElementText, Text: "gh issues "}, {Id: "issue", Kind: QueryElementBlock, Value: "owner/repo#6"}}}
	for _, text := range []string{"", "issues owner/repo#6", "other query"} {
		if normalized := hint.NormalizeForQuery("input", text); normalized != nil {
			t.Fatalf("hint substituted missing or different text %q", text)
		}
	}
	if normalized := hint.NormalizeForQuery("input", "gh issues owner/repo#6"); normalized != hint {
		t.Fatal("matching hint was discarded")
	}
	if hint.NormalizeForQuery("selection", hint.PlainText()) != nil {
		t.Fatal("selection accepted input decoration")
	}
	var absent *QueryHint
	if normalized := absent.NormalizeForQuery("input", ""); normalized != nil {
		t.Fatal("ordinary query clearing requires no hint")
	}
}

// Invalid decoration must not escape normalization even when its text matches.
func TestInvalidQueryHintIsIgnored(t *testing.T) {
	for _, hint := range []*QueryHint{
		{},
		{Elements: []QueryElement{{Id: "value", Kind: "unknown", Value: "hello"}}},
		{Elements: []QueryElement{{Id: "same", Kind: QueryElementText, Text: "hello"}, {Id: "same", Kind: QueryElementText}}},
	} {
		if hint.NormalizeForQuery("input", hint.PlainText()) != nil {
			t.Fatal("invalid hint was retained")
		}
	}
}
