module github.com/mirusu400/aram-frontend

go 1.25

require (
	github.com/ebitenui/ebitenui v0.7.3
	github.com/hajimehoshi/ebiten/v2 v2.9.9
	github.com/ncruces/zenity v0.10.14
	golang.org/x/image v0.31.0
)

require (
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/dchest/jsmin v0.0.0-20220218165748-59f39799265f // indirect
	github.com/ebitengine/gomobile v0.0.0-20250923094054-ea854a63cce1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.4.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/frustra/bbcode v0.0.0-20201127003707-6ef347fbe1c8 // indirect
	github.com/go-text/typesetting v0.3.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/josephspurrier/goversioninfo v1.4.1 // indirect
	github.com/randall77/makefat v0.0.0-20210315173500-7ddd0e42c844 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

// Ebitengine's exp/textinput indexes the GCS_COMPATTR and GCS_COMPCLAUSE
// buffers without checking their length. Hangul IMEs report both as empty, so
// composing the first syllable panics inside the window procedure. The fork
// only adds the missing length guards; drop this replace once the fix lands
// upstream. Branch: mirusu400/ebiten aram-ime-fix.
replace github.com/hajimehoshi/ebiten/v2 => github.com/mirusu400/ebiten/v2 v2.9.10-0.20260731124307-b6f31d44c846
