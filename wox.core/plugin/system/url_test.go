package system

import (
	"context"
	"path/filepath"
	"testing"

	"wox/common"
	"wox/util"

	"github.com/stretchr/testify/assert"
)

func TestUrlPlugin_Query(t *testing.T) {
	assert.True(t, util.IsUrl("https://www.google.com"))
	assert.True(t, util.IsUrl("bilibili.com"))
	assert.True(t, util.IsUrl("http://一元机场.asia"))
	assert.True(t, util.IsUrl("一元机场.asia"))

	assert.True(t, util.IsUrl("192.168.1.10"))
	assert.True(t, util.IsUrl("http://192.168.1.10"))
	assert.True(t, util.IsUrl("https://192.168.1.10"))
	assert.True(t, util.IsUrl("http://192.168.1.10:8080"))
	assert.True(t, util.IsUrl("http://192.168.1.10:8080/path"))
	assert.True(t, util.IsUrl("10.0.0.1"))
	assert.True(t, util.IsUrl("255.255.255.255"))

	// An explicit scheme matches before the host is checked.
	assert.True(t, util.IsUrl("http://google"))
	assert.True(t, util.IsUrl("http://.google.com"))
	assert.False(t, util.IsUrl("google"))
	assert.False(t, util.IsUrl(".google.com"))
}

func TestUrlPlugin_GetRecentUrlIconUsesDefaultForMissingCache(t *testing.T) {
	urlPlugin := &UrlPlugin{}
	history := UrlHistory{
		Url: "https://missing-icon-test.invalid",
		Icon: common.WoxImage{
			ImageType: common.WoxImageTypeAbsolutePath,
			ImageData: filepath.Join(t.TempDir(), "missing.png"),
		},
	}

	icon := urlPlugin.getRecentUrlIcon(context.Background(), history)

	assert.Equal(t, urlIcon, icon)
}
