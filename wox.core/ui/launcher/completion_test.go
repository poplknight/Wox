package launcher

import (
	"context"
	"testing"

	woxui "wox/ui/runtime"
)

type tabCompletionServices struct{}

func (tabCompletionServices) AcceptQueryCompletionHint(context.Context, string, string, string, string) error {
	return nil
}

func TestTabInstallsSelectedDirectoryQuery(t *testing.T) {
	app := bindTestApp(&App{
		resultList: resultController{results: []queryResult{
			{Title: "a", SubTitle: "/a", TabQuery: "cd /a/"},
			{Title: "notes.txt", SubTitle: "/notes.txt"},
		}},
		querySession: queryController{editor: woxui.NewTextEditor("cd /")},
	})
	app.querySession.services = tabCompletionServices{}
	app.querySession.query.QueryType = "input"
	app.querySession.query.QueryText = "cd /"
	app.querySession.completionEnabled = func() bool { return true }
	app.querySession.completionHint = &queryCompletionHint{InputPrefix: "cd /", CompletionText: "cd /history", Suffix: "history"}
	sent := ""
	app.querySession.sendCurrentQuery = func() error {
		sent = app.querySession.query.QueryText
		return nil
	}
	app.querySession.applyQueryTextChangeLocked = func(text string) { app.querySession.query.QueryText = text }

	if !app.onKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) {
		t.Fatal("Tab was not handled")
	}
	if got := app.querySession.editor.State().Text; got != "cd /a/" || app.querySession.query.QueryText != "cd /a/" || sent != "cd /a/" {
		t.Fatalf("query = editor %q text %q sent %q", got, app.querySession.query.QueryText, sent)
	}
	if app.querySession.completionHint != nil {
		t.Fatal("directory Tab kept the completion hint")
	}

	app.resultList.selected = 1
	app.querySession.editor.SetText("cd /", false)
	app.querySession.query.QueryText = "cd /"
	app.querySession.completionHint = &queryCompletionHint{InputPrefix: "cd /", CompletionText: "cd /history", Suffix: "history"}
	sent = ""
	if !app.onKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || app.querySession.editor.State().Text != "cd /history" || sent != "cd /history" {
		t.Fatalf("file Tab query = %q sent %q", app.querySession.editor.State().Text, sent)
	}

	app.resultList.selected = 0
	app.querySession.editor.SetText("cd /", false)
	app.querySession.query.QueryText = "cd /"
	if !app.onKey(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierShift, Down: true}) || app.querySession.editor.State().Text != "a" {
		t.Fatalf("shift+tab query = %q", app.querySession.editor.State().Text)
	}
}

// Deleting several characters before another backend response must keep the hint visible.
func TestCompletionHintRapidDeletion(t *testing.T) {
	a := bindTestApp(&App{
		querySession: queryController{
			completionHint: &queryCompletionHint{
				InputPrefix: "settin", CompletionText: "setting", Suffix: "g", DeletionReuseMinLength: 3,
			},
		},
	})
	for _, text := range []string{"setti", "sett", "set"} {
		a.querySession.reuseCompletionHintLocked(text)
		if a.querySession.completionHint == nil || a.querySession.completionHint.InputPrefix != text || text+a.querySession.completionHint.Suffix != "setting" {
			t.Fatalf("hint after deletion to %q = %#v", text, a.querySession.completionHint)
		}
	}
	a.querySession.reuseCompletionHintLocked("se")
	if a.querySession.completionHint != nil {
		t.Fatal("hint must disappear below the history minimum")
	}
}

func TestCompletionHintReuseBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, completion, text string
		minimum                        int
		want                           bool
	}{
		{"append", "set", "setting", "sett", 0, true},
		{"complete", "sett", "setting", "setting", 3, false},
		{"clear", "sett", "setting", "", 3, false},
		{"replace", "sett", "setting", "song", 3, false},
		{"plugin command", "cmd sta", "cmd start ", "cmd st", 0, false},
		{"unicode", "你好世界", "你好世界啊", "你好世", 3, true},
		{"unicode minimum", "你好世", "你好世界", "你好", 3, false},
		{"whitespace minimum", "  sett", "  setting", "  se", 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := bindTestApp(&App{querySession: queryController{completionHint: &queryCompletionHint{InputPrefix: tc.prefix, CompletionText: tc.completion, DeletionReuseMinLength: tc.minimum}}})
			a.querySession.reuseCompletionHintLocked(tc.text)
			if (a.querySession.completionHint != nil) != tc.want {
				t.Fatalf("hint = %#v, want present %t", a.querySession.completionHint, tc.want)
			}
		})
	}
}
