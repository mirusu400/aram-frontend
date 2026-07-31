package main

// icon.ico holds the executable icon that Windows Explorer, the taskbar and
// the Alt-Tab switcher show. The Go linker picks up rsrc_windows_amd64.syso
// automatically for windows/amd64 builds; nothing imports it.
//
// icon.ico is built from frontend/assets/icon.png, the same artwork the
// running window uses through ebiten.SetWindowIcon. After changing that PNG,
// rebuild both: produce a multi size icon.ico from it (16, 24, 32, 48, 64 and
// 128 pixel entries, reduced with nearest neighbour so the pixel art keeps its
// edges), then regenerate the resource object with
//
//	go run github.com/akavel/rsrc@v0.10.2 \
//	    -ico cmd/aram-frontend/icon.ico -arch amd64 \
//	    -o cmd/aram-frontend/rsrc_windows_amd64.syso
