package setting

import (
	"os"
	"path/filepath"
	"testing"

	"wox/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestSettingsFileRoundTripReplacesEverySetting(t *testing.T) {
	db := openSettingsFileTestDB(t)
	woxStore := NewWoxSettingStore(db)
	if err := woxStore.Set("ThemeId", "dark"); err != nil {
		t.Fatalf("save theme: %v", err)
	}
	if err := woxStore.Set("LogLevel", "DEBUG"); err != nil {
		t.Fatalf("save log level: %v", err)
	}
	pluginStore := NewPluginSettingStore(db, "clipboard")
	if err := pluginStore.set("HistoryLimit", "100", false); err != nil {
		t.Fatalf("save plugin setting: %v", err)
	}
	localStore := NewPluginSettingStore(db, "notes")
	if err := localStore.set("Draft", "keep", true); err != nil {
		t.Fatalf("save local plugin setting: %v", err)
	}

	document, err := ReadSettingsDocument(db)
	if err != nil {
		t.Fatalf("read document: %v", err)
	}
	path := filepath.Join(t.TempDir(), "wox-settings.woxconfig")
	if err := WriteSettingsFile(path, document); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := woxStore.Set("ThemeId", "light"); err != nil {
		t.Fatalf("change theme: %v", err)
	}
	if err := woxStore.Set("Extra", "gone"); err != nil {
		t.Fatalf("add extra setting: %v", err)
	}
	if err := pluginStore.set("HistoryLimit", "1", false); err != nil {
		t.Fatalf("change plugin setting: %v", err)
	}
	if err := NewPluginSettingStore(db, "other").set("Enabled", "true", false); err != nil {
		t.Fatalf("add other plugin setting: %v", err)
	}

	loaded, err := LoadSettingsFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	result, err := ApplySettingsDocument(db, loaded, nil)
	if err != nil {
		t.Fatalf("apply file: %v", err)
	}
	if !result.ThemeChanged {
		t.Fatal("theme change was not reported")
	}

	restored, err := ReadSettingsDocument(db)
	if err != nil {
		t.Fatalf("reread document: %v", err)
	}
	if !sameSettingsFile(document, restored) {
		t.Fatalf("restored settings = %#v, want %#v", restored, document)
	}
}

func TestLoadSettingsFileRejectsUnknownFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "other.woxconfig")
	if err := os.WriteFile(path, []byte(`{"format":"other","version":1}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := LoadSettingsFile(path); err == nil {
		t.Fatal("unknown format was accepted")
	}
}

func openSettingsFileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&database.WoxSetting{}, &database.PluginSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func sameSettingsFile(left SettingsFile, right SettingsFile) bool {
	if len(left.WoxSettings) != len(right.WoxSettings) || len(left.PluginSettings) != len(right.PluginSettings) {
		return false
	}
	for index := range left.WoxSettings {
		if left.WoxSettings[index] != right.WoxSettings[index] {
			return false
		}
	}
	for index := range left.PluginSettings {
		if left.PluginSettings[index] != right.PluginSettings[index] {
			return false
		}
	}
	return true
}
