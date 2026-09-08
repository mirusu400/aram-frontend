package frontend

import (
	"context"
	"testing"
	"time"
)

// TestReopeningARecentInputKeepsItsName is the regression for a title that
// renamed itself to a UUID. A mobile host imports into private storage under a
// generated file name; opening from Recent or Home dropped the name the host
// had reported, the backend fell back to the path's base name, and addRecent
// then wrote that generated name back over the good one - permanently, and
// into every save backup name and issue report title afterwards.
func TestReopeningARecentInputKeepsItsName(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	const (
		importPath = "/data/user/0/app/files/imports/9f2c/이노티아1.zip"
		realName   = "이노티아1.zip"
	)

	backend := &openRecordingBackend{requests: make(chan OpenRequest, 2)}
	shell := &Shell{
		backend:          backend,
		settings:         defaultSettings(),
		backendResults:   make(chan backendResult, 2),
		openStageResults: make(chan OpenStage, 4),
	}
	shell.settings.Language = string(LanguageEnglish)
	shell.settings.addRecent(importPath, realName, "93d5b6b8")

	for _, open := range []struct {
		name string
		run  func()
	}{
		{"recent", func() { shell.openRecentPath(importPath) }},
		{"home", func() { shell.homeOpenPath(importPath) }},
	} {
		t.Run(open.name, func(t *testing.T) {
			shell.loading = false
			open.run()
			select {
			case request := <-backend.requests:
				if request.DisplayName != realName {
					t.Fatalf("DisplayName = %q, want %q", request.DisplayName, realName)
				}
			case <-time.After(time.Second):
				t.Fatal("the backend never received the open request")
			}
		})
	}
	_ = context.Background()
}

func TestRememberedDisplayNameIgnoresUnknownPaths(t *testing.T) {
	shell := &Shell{settings: defaultSettings()}
	shell.settings.addRecent("/imports/9f2c/title.zip", "title.zip", "aa")
	if name := shell.rememberedDisplayName("/elsewhere/title.zip"); name != "" {
		t.Fatalf("rememberedDisplayName = %q, want empty for an unknown path", name)
	}
}
