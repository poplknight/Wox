package info

import (
	"context"
	"net/netip"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Plugin{})
}

// Plugin shows local machine details behind the info command.
type Plugin struct {
	api       plugin.API
	lifecycle context.Context
	samples   sampleState

	mu            sync.Mutex
	viewGen       int
	viewCommand   string
	publicV4      string
	publicV6      string
	publicAt      time.Time
	publicLoading bool
	publicFailed  bool
}

func (p *Plugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              common.InfoPluginID,
		Name:            "i18n:plugin_info_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_info_plugin_description",
		Icon:            icons.Get(icons.PluginSys).String(),
		TriggerKeywords: []string{"info"},
		Features: []plugin.MetadataFeature{
			{Name: plugin.MetadataFeatureIgnoreAutoScore},
		},
		SupportedOS: []string{"Windows", "Macos", "Linux"},
	}
}

func (p *Plugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
	p.lifecycle = ctx
	util.Go(ctx, "info hardware profile", func() {
		warmHardwareProfile(ctx)
		p.refreshAfterHardwareProfile(ctx)
	})
}

func (p *Plugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	command, search := splitInfoQuery(query.Command, query.Search)
	generation := p.noteView(command)
	snap := collectSnapshot(ctx, &p.samples, time.Now())
	if command == commandIP {
		p.attachPublicIP(&snap, generation)
	}
	rows := infoRows(snap, command, search, p.text(ctx))
	results := make([]plugin.QueryResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, p.result(ctx, query, row))
	}
	if liveInfoCommand(command) {
		p.scheduleRefresh(generation, command)
	}
	return plugin.NewQueryResponse(results)
}

func (p *Plugin) result(ctx context.Context, query plugin.Query, row infoRow) plugin.QueryResult {
	actions := []plugin.QueryResultAction{}
	if row.Drill != "" {
		command := row.Drill
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_info_open",
			Icon:                   icons.Get(icons.ActionOpen),
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, _ plugin.ActionContext) {
				p.api.ChangeQuery(ctx, common.PlainQuery{
					QueryType: plugin.QueryTypeInput,
					QueryText: strings.TrimSpace(query.TriggerKeyword) + " " + command + " ",
				})
			},
		})
	}
	if row.Copy != "" {
		text := row.Copy
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_info_copy",
			Icon:                   icons.Get(icons.ActionCopy),
			IsDefault:              row.Drill == "",
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, _ plugin.ActionContext) {
				p.api.Copy(ctx, plugin.CopyParams{Type: plugin.CopyTypePlainText, Text: text})
				p.api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: "i18n:plugin_info_copied"})
			},
		})
	}
	return plugin.QueryResult{
		Title:       row.Title,
		SubTitle:    row.Subtitle,
		IdentityKey: row.Identity,
		Score:       row.Score,
		Icon:        icons.Get(icons.PluginSys),
		Actions:     actions,
	}
}

func (p *Plugin) text(ctx context.Context) infoText {
	return func(key string) string {
		if p.api == nil {
			return key
		}
		return p.api.GetTranslation(ctx, key)
	}
}

func (p *Plugin) noteView(command string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.viewGen++
	p.viewCommand = command
	return p.viewGen
}

// refreshAfterHardwareProfile replaces the raw model identifier once the marketing name is available.
func (p *Plugin) refreshAfterHardwareProfile(ctx context.Context) {
	p.mu.Lock()
	queried := p.viewGen > 0
	command := p.viewCommand
	p.mu.Unlock()
	if !queried || (command != "" && command != commandMachine) || p.api == nil || !p.api.IsVisible(ctx) {
		return
	}
	p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
}

func (p *Plugin) viewCurrent(generation int, command string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.viewGen == generation && p.viewCommand == command
}

func (p *Plugin) scheduleRefresh(generation int, command string) {
	ctx := p.lifecycle
	if ctx == nil {
		ctx = context.Background()
	}
	util.Go(ctx, "info refresh", func() {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if p.api == nil || !p.viewCurrent(generation, command) || !p.api.IsVisible(ctx) {
			return
		}
		p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
	})
}

func (p *Plugin) attachPublicIP(snap *infoSnapshot, generation int) {
	p.mu.Lock()
	fresh := !p.publicAt.IsZero() && time.Since(p.publicAt) < 5*time.Minute
	switch {
	case fresh && (p.publicV4 != "" || p.publicV6 != ""):
		snap.PublicIPv4 = p.publicV4
		snap.PublicIPv6 = p.publicV6
		snap.PublicState = publicIPReady
		p.mu.Unlock()
		return
	case fresh && p.publicFailed:
		snap.PublicState = publicIPFailed
		p.mu.Unlock()
		return
	case p.publicLoading:
		snap.PublicState = publicIPLoading
		p.mu.Unlock()
		return
	default:
		p.publicLoading = true
		snap.PublicState = publicIPLoading
		p.mu.Unlock()
	}
	ctx := p.lifecycle
	if ctx == nil {
		ctx = context.Background()
	}
	util.Go(ctx, "info public address", func() {
		v4, v6, failed := fetchPublicAddresses(ctx)
		p.mu.Lock()
		p.publicLoading = false
		p.publicAt = time.Now()
		p.publicV4 = v4
		p.publicV6 = v6
		p.publicFailed = failed && v4 == "" && v6 == ""
		p.mu.Unlock()
		if p.api != nil && p.viewCurrent(generation, commandIP) && p.api.IsVisible(ctx) {
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		}
	})
}

func fetchPublicAddresses(ctx context.Context) (string, string, bool) {
	requestCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	v4, v4Err := fetchPublicAddress(requestCtx, "https://api.ipify.org")
	v6, v6Err := fetchPublicAddress(requestCtx, "https://api6.ipify.org")
	return v4, v6, v4Err != nil && v6Err != nil
}

func fetchPublicAddress(ctx context.Context, endpoint string) (string, error) {
	body, err := util.HttpGet(ctx, endpoint)
	if err != nil {
		return "", err
	}
	address := strings.TrimSpace(string(body))
	if _, err := netip.ParseAddr(address); err != nil {
		return "", err
	}
	return address, nil
}

func liveInfoCommand(command string) bool {
	return command == commandCPU || command == commandMemory || command == commandNet
}
