package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strconv"
	"strings"

	"wox/common"
	"wox/common/icons"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/supervisor"
	"wox/ui/contract"
	"wox/util"
	"wox/util/shell"
)

// DataLocation returns the active user-data directory.
func (s *CoreServices) DataLocation(_ context.Context, _ string) (string, error) {
	return util.GetLocation().GetUserDataDirectory(), nil
}

// DataBackups returns all settings backups in their persisted representation.
func (s *CoreServices) DataBackups(ctx context.Context, sessionID string) ([]contract.DataBackup, error) {
	backups, err := setting.GetSettingManager().FindAllBackups(uiServiceContext(ctx, sessionID))
	if err != nil {
		return nil, err
	}
	converted := make([]contract.DataBackup, len(backups))
	for index, backup := range backups {
		converted[index] = contract.DataBackup{
			ID:        backup.Id,
			Name:      backup.Name,
			Timestamp: backup.Timestamp,
			Type:      string(backup.Type),
			Path:      backup.Path,
		}
	}
	return converted, nil
}

// CreateDataBackup creates one manual settings backup.
func (s *CoreServices) CreateDataBackup(ctx context.Context, sessionID string) error {
	err := setting.GetSettingManager().Backup(uiServiceContext(ctx, sessionID), setting.BackupTypeManual)
	if errors.Is(err, setting.ErrBackupInProgress) {
		return contract.ErrBackupInProgress
	}
	return err
}

// WatchDataBackup follows the single in-flight backup shared by manual and automatic runs.
func (s *CoreServices) WatchDataBackup(_ context.Context, _ string, onChange func(running bool, failure error)) func() {
	if onChange == nil {
		return func() {}
	}
	return setting.GetSettingManager().WatchBackup(onChange)
}

// RestoreDataBackup asks the supervisor to replace user data after this process exits.
func (s *CoreServices) RestoreDataBackup(ctx context.Context, sessionID string, backupID string) error {
	return RequestUserDataRestore(uiServiceContext(ctx, sessionID), backupID)
}

// DeleteDataBackup removes one finished backup directory.
func (s *CoreServices) DeleteDataBackup(ctx context.Context, sessionID string, backupID string) error {
	return setting.GetSettingManager().DeleteBackup(uiServiceContext(ctx, sessionID), backupID)
}

// RequestUserDataRestore asks the supervisor to replace user data after this process exits.
func RequestUserDataRestore(ctx context.Context, backupID string) error {
	backups, err := setting.GetSettingManager().FindAllBackups(ctx)
	if err != nil {
		return err
	}
	var backupPath string
	for _, backup := range backups {
		if backup.Id == backupID {
			backupPath = backup.Path
			break
		}
	}
	if backupPath == "" {
		return fmt.Errorf("backup not found: %s", backupID)
	}
	payload, err := json.Marshal(map[string]string{
		"backupPath":  backupPath,
		"userDataDir": util.GetLocation().GetUserDataDirectory(),
	})
	if err != nil {
		return err
	}
	if err := supervisor.Submit(ctx, supervisor.Task{
		Type:     supervisor.TaskRestore,
		RunAfter: supervisor.RunAfterExit,
		Payload:  payload,
	}); err != nil {
		return err
	}
	GetUIManager().ExitApp(ctx)
	return nil
}

// ChangeDataLocation moves user-managed data through the core UI manager.
func (s *CoreServices) ChangeDataLocation(ctx context.Context, sessionID string, location string) error {
	return GetUIManager().ChangeUserDataDirectory(uiServiceContext(ctx, sessionID), location)
}

// ClearLogs removes historical logs and preserves the existing user notification behavior.
func (s *CoreServices) ClearLogs(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	if err := util.GetLogger().ClearHistory(filepath.Base(supervisor.GetManager().SupervisorLogPath())); err != nil {
		GetUIManager().GetUI(ctx).Notify(ctx, common.NotifyMsg{
			Icon:           icons.Get(icons.BrandWox).String(),
			Text:           fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "ui_data_log_clear_notify_failed"), err.Error()),
			DisplaySeconds: 6,
		})
		return err
	}
	GetUIManager().GetUI(ctx).Notify(ctx, common.NotifyMsg{
		Icon:           icons.Get(icons.BrandWox).String(),
		Text:           i18n.GetI18nManager().TranslateWox(ctx, "ui_data_log_clear_notify_success"),
		DisplaySeconds: 4,
	})
	return nil
}

// OpenPath delegates path opening to the platform shell adapter.
func (s *CoreServices) OpenPath(_ context.Context, _ string, path string) error {
	return shell.Open(path)
}

// BackupFolder ensures and returns the configured backup directory.
func (s *CoreServices) BackupFolder(_ context.Context, _ string) (string, error) {
	backupDir := util.GetLocation().GetBackupDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(backupDir); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}
	return backupDir, nil
}

// ExportSettingsFile writes every Wox and plugin setting to path.
func (s *CoreServices) ExportSettingsFile(_ context.Context, _ string, path string) error {
	document, err := setting.ReadSettingsDocument(database.GetDB())
	if err != nil {
		return err
	}
	return setting.WriteSettingsFile(path, document)
}

// ImportSettingsFile replaces current settings with the file and applies them immediately.
func (s *CoreServices) ImportSettingsFile(ctx context.Context, sessionID string, path string) error {
	ctx = uiServiceContext(ctx, sessionID)
	document, err := setting.LoadSettingsFile(path)
	if err != nil {
		return err
	}
	result, err := setting.ApplySettingsDocument(database.GetDB(), document, setting.CurrentWoxSetting())
	if err != nil {
		return err
	}
	return applyImportedSettings(ctx, result)
}

// applyImportedSettings pushes imported values into hotkeys, plugins, theme, and language.
func applyImportedSettings(ctx context.Context, result setting.SettingsFileApplyResult) error {
	uiManager := GetUIManager()
	for _, change := range result.Wox {
		uiManager.PostSettingUpdate(ctx, change.Key, change.Value)
	}
	if err := applyImportedPluginSettings(ctx, result.Plugins); err != nil {
		return err
	}
	if result.ThemeChanged {
		uiManager.ApplyCurrentTheme(ctx)
	}
	return nil
}

func applyImportedPluginSettings(ctx context.Context, changes []setting.PluginSettingsFileChange) error {
	if len(changes) == 0 {
		return nil
	}
	instances := plugin.GetPluginManager().GetPluginInstances()
	byID := make(map[string]*plugin.Instance, len(instances))
	for _, instance := range instances {
		if instance != nil {
			byID[instance.Metadata.Id] = instance
		}
	}
	for _, change := range changes {
		instance := byID[change.PluginID]
		key := importedPluginSettingKey(change.Key)
		if instance != nil && instance.Setting != nil {
			if err := applyImportedPluginRuntimeSetting(ctx, instance, key, change); err != nil {
				return err
			}
		}
		if instance == nil {
			continue
		}
		value := change.Value
		if change.Deleted {
			value = ""
		}
		for _, callback := range instance.SettingChangeCallbacks {
			callback(ctx, key, value)
		}
	}
	return nil
}

// applyImportedPluginRuntimeSetting refreshes settings the process already cached,
// and starts or stops a plugin when its disabled flag changes.
func applyImportedPluginRuntimeSetting(ctx context.Context, instance *plugin.Instance, key string, change setting.PluginSettingsFileChange) error {
	switch key {
	case "Disabled":
		wantDisabled := false
		if !change.Deleted {
			parsed, err := strconv.ParseBool(strings.TrimSpace(change.Value))
			if err != nil {
				return fmt.Errorf("plugin %s disabled flag: %w", change.PluginID, err)
			}
			wantDisabled = parsed
		}
		if instance.Setting.Disabled == nil || instance.Setting.Disabled.Get() == wantDisabled {
			return nil
		}
		if wantDisabled {
			return plugin.GetPluginManager().DisablePlugin(ctx, change.PluginID)
		}
		return plugin.GetPluginManager().EnablePlugin(ctx, change.PluginID)
	case "TriggerKeywords":
		if instance.Setting.TriggerKeywords == nil {
			return nil
		}
		if change.Deleted {
			instance.Setting.TriggerKeywords.ResetCached()
			return nil
		}
		return instance.Setting.TriggerKeywords.SetCachedFromString(change.Value)
	default:
		return nil
	}
}

func importedPluginSettingKey(key string) string {
	if baseKey, _, ok := setting.SplitPlatformSettingKey(key); ok {
		return baseKey
	}
	return key
}

// OpenLog creates and reveals the current log file.
func (s *CoreServices) OpenLog(_ context.Context, _ string) error {
	logFile := util.GetLogger().CurrentLogPath()
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return shell.OpenFileInFolder(logFile)
}
