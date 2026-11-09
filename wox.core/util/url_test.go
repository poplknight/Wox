package util

import (
	"strings"
	"testing"
)

func TestIsUrl(t *testing.T) {
	accepted := []string{
		"example.com",
		" https://example.com/path ",
		"127.0.0.1:8080/path",
		"http://localhost/path.with.dot",
		"http://一元机场.asia",
		"HTTPS://一元机场.asia/path?q=1",
		"一元机场.asia",
		"Example.COM",
		"example.museum",
		"github.com/Wox-launcher/Wox",
		"google.com/search?q=wox",
		"一元机场.asia:8443",
	}
	for _, value := range accepted {
		if !IsUrl(value) {
			t.Fatalf("IsUrl(%q) = false, want true", value)
		}
	}

	rejected := []string{
		"",
		"localhost",
		"google",
		".google.com",
		"hello world. example",
		"example.com/a\nb",
		"example.c",
		"example.com:0",
		"example.com:65536",
		strings.Repeat("a", 128*1024),
		strings.Repeat("a", 128*1024) + ".com",
	}
	for _, value := range rejected {
		if IsUrl(value) {
			t.Fatalf("IsUrl(%q) = true, want false", value)
		}
	}

	longScheme := "http://" + strings.Repeat("a", 128*1024)
	if !IsUrl(longScheme) {
		t.Fatal("explicit http scheme must match before host validation")
	}
}

func TestNormalizeUrlKeepsExplicitScheme(t *testing.T) {
	if got := NormalizeUrl(" HTTP://一元机场.asia "); got != "HTTP://一元机场.asia" {
		t.Fatalf("NormalizeUrl explicit scheme = %q", got)
	}
	if got := NormalizeUrl("一元机场.asia"); got != "https://一元机场.asia" {
		t.Fatalf("NormalizeUrl bare domain = %q", got)
	}
}
