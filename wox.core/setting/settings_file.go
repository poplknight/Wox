package setting

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"

	"wox/database"

	"gorm.io/gorm"
)

const (
	// SettingsFileFormat identifies a portable Wox settings file.
	SettingsFileFormat = "wox-settings"
	// SettingsFileVersion is the only file version this build reads and writes.
	SettingsFileVersion = 1
	// settingsFileMaxBytes keeps a hand-picked file from loading an unbounded document.
	settingsFileMaxBytes = 32 << 20
)

// SettingsFile is the on-disk document produced by Sync and consumed by Import.
type SettingsFile struct {
	Format         string                   `json:"format"`
	Version        int                      `json:"version"`
	WoxSettings    []SettingsFileItem       `json:"woxSettings"`
	PluginSettings []PluginSettingsFileItem `json:"pluginSettings"`
}

// SettingsFileItem is one stored Wox setting, using the same text the database keeps.
type SettingsFileItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PluginSettingsFileItem is one stored plugin setting.
type PluginSettingsFileItem struct {
	PluginID string `json:"pluginId"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsLocal  bool   `json:"isLocal"`
}

// SettingsFileChange is one Wox setting the import wrote or removed.
// Value is the effective stored text after the change, including a restored default.
type SettingsFileChange struct {
	Key     string
	Value   string
	Deleted bool
}

// PluginSettingsFileChange is one plugin setting the import wrote or removed.
type PluginSettingsFileChange struct {
	PluginID string
	Key      string
	Value    string
	Deleted  bool
}

// SettingsFileApplyResult lists the settings that actually changed.
type SettingsFileApplyResult struct {
	Wox          []SettingsFileChange
	Plugins      []PluginSettingsFileChange
	ThemeChanged bool
}

type liveSetting interface {
	Key() string
	SetFromString(string) error
	DeleteLocal() error
	StoredString() (string, error)
}

// ReadSettingsDocument copies every Wox and plugin setting row into a file document.
func ReadSettingsDocument(db *gorm.DB) (SettingsFile, error) {
	if db == nil {
		return SettingsFile{}, fmt.Errorf("database is not initialized")
	}
	var woxRows []database.WoxSetting
	if err := db.Find(&woxRows).Error; err != nil {
		return SettingsFile{}, fmt.Errorf("read wox settings: %w", err)
	}
	var pluginRows []database.PluginSetting
	if err := db.Find(&pluginRows).Error; err != nil {
		return SettingsFile{}, fmt.Errorf("read plugin settings: %w", err)
	}

	file := SettingsFile{
		Format:         SettingsFileFormat,
		Version:        SettingsFileVersion,
		WoxSettings:    make([]SettingsFileItem, 0, len(woxRows)),
		PluginSettings: make([]PluginSettingsFileItem, 0, len(pluginRows)),
	}
	for _, row := range woxRows {
		if row.Key == "" {
			continue
		}
		file.WoxSettings = append(file.WoxSettings, SettingsFileItem{Key: row.Key, Value: row.Value})
	}
	for _, row := range pluginRows {
		if row.PluginID == "" || row.Key == "" {
			continue
		}
		file.PluginSettings = append(file.PluginSettings, PluginSettingsFileItem{
			PluginID: row.PluginID, Key: row.Key, Value: row.Value, IsLocal: row.IsLocal,
		})
	}
	sort.Slice(file.WoxSettings, func(i, j int) bool { return file.WoxSettings[i].Key < file.WoxSettings[j].Key })
	sort.Slice(file.PluginSettings, func(i, j int) bool {
		if file.PluginSettings[i].PluginID == file.PluginSettings[j].PluginID {
			return file.PluginSettings[i].Key < file.PluginSettings[j].Key
		}
		return file.PluginSettings[i].PluginID < file.PluginSettings[j].PluginID
	})
	return file, nil
}

// WriteSettingsFile saves a settings document as JSON.
func WriteSettingsFile(path string, file SettingsFile) error {
	if file.Format == "" {
		file.Format = SettingsFileFormat
	}
	if file.Version == 0 {
		file.Version = SettingsFileVersion
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings file: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}
	return nil
}

// LoadSettingsFile reads a document written by WriteSettingsFile.
func LoadSettingsFile(path string) (SettingsFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return SettingsFile{}, fmt.Errorf("open settings file: %w", err)
	}
	if info.Size() > settingsFileMaxBytes {
		return SettingsFile{}, fmt.Errorf("settings file is larger than %d bytes", settingsFileMaxBytes)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return SettingsFile{}, fmt.Errorf("read settings file: %w", err)
	}
	var file SettingsFile
	if err := json.Unmarshal(encoded, &file); err != nil {
		return SettingsFile{}, fmt.Errorf("decode settings file: %w", err)
	}
	if file.Format != SettingsFileFormat || file.Version != SettingsFileVersion {
		return SettingsFile{}, fmt.Errorf("unsupported settings file format %q version %d", file.Format, file.Version)
	}
	return file, nil
}

// ApplySettingsDocument replaces stored settings with the file.
// Keys absent from the file are removed. live may be nil when settings are not loaded yet;
// recognized Wox settings are then written only to the database.
func ApplySettingsDocument(db *gorm.DB, file SettingsFile, live *WoxSetting) (SettingsFileApplyResult, error) {
	if db == nil {
		return SettingsFileApplyResult{}, fmt.Errorf("database is not initialized")
	}
	if file.Format != SettingsFileFormat || file.Version != SettingsFileVersion {
		return SettingsFileApplyResult{}, fmt.Errorf("unsupported settings file format %q version %d", file.Format, file.Version)
	}

	var result SettingsFileApplyResult
	woxStore := NewWoxSettingStore(db)
	woxChanges, themeChanged, err := applyWoxSettingsFile(woxStore, file.WoxSettings, live)
	if err != nil {
		return SettingsFileApplyResult{}, err
	}
	pluginChanges, err := applyPluginSettingsFile(db, file.PluginSettings)
	if err != nil {
		return SettingsFileApplyResult{}, err
	}
	result.Wox = woxChanges
	result.Plugins = pluginChanges
	result.ThemeChanged = themeChanged
	return result, nil
}

func applyWoxSettingsFile(store *WoxSettingStore, items []SettingsFileItem, live *WoxSetting) ([]SettingsFileChange, bool, error) {
	var rows []database.WoxSetting
	if err := store.db.Find(&rows).Error; err != nil {
		return nil, false, fmt.Errorf("read wox settings: %w", err)
	}
	current := make(map[string]string, len(rows))
	for _, row := range rows {
		current[row.Key] = row.Value
	}
	wanted := make(map[string]string, len(items))
	for _, item := range items {
		if item.Key == "" {
			continue
		}
		wanted[item.Key] = item.Value
	}

	var changes []SettingsFileChange
	themeChanged := false
	for key := range current {
		if _, ok := wanted[key]; ok {
			continue
		}
		value, err := deleteWoxSettingValue(store, live, key)
		if err != nil {
			return nil, false, err
		}
		changes = append(changes, SettingsFileChange{Key: key, Value: value, Deleted: true})
		if key == "ThemeId" {
			themeChanged = true
		}
	}
	for key, value := range wanted {
		if previous, ok := current[key]; ok && previous == value {
			continue
		}
		effective, err := upsertWoxSettingValue(store, live, key, value)
		if err != nil {
			return nil, false, err
		}
		changes = append(changes, SettingsFileChange{Key: key, Value: effective, Deleted: false})
		if key == "ThemeId" {
			themeChanged = true
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Key < changes[j].Key })
	return changes, themeChanged, nil
}

func applyPluginSettingsFile(db *gorm.DB, items []PluginSettingsFileItem) ([]PluginSettingsFileChange, error) {
	var rows []database.PluginSetting
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("read plugin settings: %w", err)
	}
	type pluginKey struct {
		pluginID string
		key      string
	}
	current := make(map[pluginKey]database.PluginSetting, len(rows))
	for _, row := range rows {
		current[pluginKey{row.PluginID, row.Key}] = row
	}
	wanted := make(map[pluginKey]PluginSettingsFileItem, len(items))
	for _, item := range items {
		if item.PluginID == "" || item.Key == "" {
			continue
		}
		wanted[pluginKey{item.PluginID, item.Key}] = item
	}

	var changes []PluginSettingsFileChange
	for key, row := range current {
		if _, ok := wanted[key]; ok {
			continue
		}
		store := NewPluginSettingStore(db, row.PluginID)
		if err := store.Delete(row.Key); err != nil {
			return nil, fmt.Errorf("delete plugin setting %s/%s: %w", row.PluginID, row.Key, err)
		}
		changes = append(changes, PluginSettingsFileChange{PluginID: row.PluginID, Key: row.Key, Deleted: true})
	}
	for key, item := range wanted {
		if previous, ok := current[key]; ok && previous.Value == item.Value && previous.IsLocal == item.IsLocal {
			continue
		}
		store := NewPluginSettingStore(db, item.PluginID)
		if err := store.set(item.Key, item.Value, item.IsLocal); err != nil {
			return nil, fmt.Errorf("save plugin setting %s/%s: %w", item.PluginID, item.Key, err)
		}
		changes = append(changes, PluginSettingsFileChange{
			PluginID: item.PluginID, Key: item.Key, Value: item.Value, Deleted: false,
		})
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].PluginID == changes[j].PluginID {
			return changes[i].Key < changes[j].Key
		}
		return changes[i].PluginID < changes[j].PluginID
	})
	return changes, nil
}

func upsertWoxSettingValue(store *WoxSettingStore, live *WoxSetting, key string, value string) (string, error) {
	if settingValue, ok := findLiveSetting(live, key); ok {
		if err := settingValue.SetFromString(value); err != nil {
			return "", fmt.Errorf("apply setting %s: %w", key, err)
		}
		stored, err := settingValue.StoredString()
		if err != nil {
			return value, nil
		}
		return stored, nil
	}
	if err := store.Set(key, value); err != nil {
		return "", fmt.Errorf("save setting %s: %w", key, err)
	}
	return value, nil
}

func deleteWoxSettingValue(store *WoxSettingStore, live *WoxSetting, key string) (string, error) {
	if settingValue, ok := findLiveSetting(live, key); ok {
		if err := settingValue.DeleteLocal(); err != nil {
			return "", fmt.Errorf("delete setting %s: %w", key, err)
		}
		stored, err := settingValue.StoredString()
		if err != nil {
			return "", nil
		}
		return stored, nil
	}
	if err := store.Delete(key); err != nil {
		return "", fmt.Errorf("delete setting %s: %w", key, err)
	}
	return "", nil
}

func findLiveSetting(woxSetting *WoxSetting, key string) (liveSetting, bool) {
	if woxSetting == nil {
		return nil, false
	}
	value := reflect.ValueOf(woxSetting).Elem()
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		settingValue, ok := field.Interface().(liveSetting)
		if !ok || settingValue.Key() != key {
			continue
		}
		return settingValue, true
	}
	return nil, false
}
