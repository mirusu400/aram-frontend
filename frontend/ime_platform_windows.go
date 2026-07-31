//go:build windows

package frontend

import (
	"image"
	"sync"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/sys/windows"
)

// Ebitengine detaches the IME from its window on creation and only reattaches
// it while an exp/textinput session runs. That session cannot be used here:
// its Windows composition handler indexes the GCS_COMPCLAUSE buffer without
// checking its length, and every Hangul IME reports no clauses, so the first
// composed syllable crashes the process inside the window procedure
// (ebiten v2.9.9 exp/textinput/textinput_windows.go:266, still present in
// v2.10.0-alpha.13; reproducible with Ebitengine's own textinput example).
//
// Reattaching the input context directly avoids that code path completely.
// The system IME then owns composition and delivers the committed text as
// WM_CHAR, which reaches the frontend through ebiten.AppendInputChars.
const platformIMEUsesEbitenTextInput = false

// glfwWindowClass is the class Ebitengine's bundled GLFW registers for its
// windows.
const glfwWindowClass = "GLFW30"

const (
	iaceDefault = 0x0010
	cfsPoint    = 0x0002
)

type win32Point struct {
	x int32
	y int32
}

type win32Rect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

type win32CompositionForm struct {
	style      uint32
	currentPos win32Point
	area       win32Rect
}

var (
	imm32  = windows.NewLazySystemDLL("imm32.dll")
	user32 = windows.NewLazySystemDLL("user32.dll")

	procImmAssociateContext     = imm32.NewProc("ImmAssociateContext")
	procImmAssociateContextEx   = imm32.NewProc("ImmAssociateContextEx")
	procImmGetContext           = imm32.NewProc("ImmGetContext")
	procImmReleaseContext       = imm32.NewProc("ImmReleaseContext")
	procImmSetCompositionWindow = imm32.NewProc("ImmSetCompositionWindow")

	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
)

var gameWindowSearch struct {
	once     sync.Once
	callback uintptr
	found    windows.HWND
}

// gameWindow locates the visible top level window this process owns. The
// lookup is retried until the window exists, because text input may be
// requested before Ebitengine has shown it.
func gameWindow() windows.HWND {
	if gameWindowSearch.found != 0 {
		return gameWindowSearch.found
	}
	gameWindowSearch.once.Do(func() {
		gameWindowSearch.callback = windows.NewCallback(visitTopLevelWindow)
	})
	if gameWindowSearch.callback == 0 {
		return 0
	}
	//nolint:errcheck // EnumWindows reports the callback's stop request as an error.
	procEnumWindows.Call(gameWindowSearch.callback, 0)
	return gameWindowSearch.found
}

func visitTopLevelWindow(handle windows.HWND, _ uintptr) uintptr {
	var owner uint32
	procGetWindowThreadProcessID.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&owner)),
	)
	if owner != windows.GetCurrentProcessId() {
		return 1
	}
	if visible, _, _ := procIsWindowVisible.Call(uintptr(handle)); visible == 0 {
		return 1
	}
	name := make([]uint16, 64)
	length, _, _ := procGetClassNameW.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&name[0])),
		uintptr(len(name)),
	)
	if length == 0 || windows.UTF16ToString(name[:length]) != glfwWindowClass {
		return 1
	}
	gameWindowSearch.found = handle
	return 0
}

// IMM calls are serviced by the thread that owns the window, and Ebitengine
// only lets that thread pump messages between frames. Calling them from the
// goroutine that runs Update therefore deadlocks the game loop, so the desired
// state is published here and applied by a worker instead.
var imeState = struct {
	signal chan struct{}
	worker sync.Once

	mu       sync.Mutex
	enabled  bool
	caret    image.Rectangle
	attached bool
}{
	signal: make(chan struct{}, 1),
}

// platformIMEAttached reports whether the window currently owns an input
// context. Without one the IME is detached and composed scripts cannot be
// typed at all, so this is the signal worth asserting on.
func platformIMEAttached() bool {
	imeState.mu.Lock()
	defer imeState.mu.Unlock()
	return imeState.attached
}

func focusPlatformIME(caret image.Rectangle) {
	requestPlatformIME(true, caret)
}

func blurPlatformIME() {
	requestPlatformIME(false, image.Rectangle{})
}

// movePlatformIMECaret keeps the composition and candidate windows next to the
// caret.
func movePlatformIMECaret(caret image.Rectangle) {
	imeState.mu.Lock()
	enabled := imeState.enabled
	imeState.mu.Unlock()
	if !enabled {
		return
	}
	requestPlatformIME(true, caret)
}

func requestPlatformIME(enabled bool, caret image.Rectangle) {
	imeState.mu.Lock()
	unchanged := imeState.enabled == enabled && imeState.caret == caret
	imeState.enabled = enabled
	imeState.caret = caret
	imeState.mu.Unlock()
	if unchanged {
		return
	}

	imeState.worker.Do(func() { go applyPlatformIMEState() })
	select {
	case imeState.signal <- struct{}{}:
	default:
		// A pass is already pending and will read the newest state.
	}
}

func applyPlatformIMEState() {
	var appliedEnabled bool
	var appliedCaret image.Rectangle
	for range imeState.signal {
		imeState.mu.Lock()
		enabled, caret := imeState.enabled, imeState.caret
		imeState.mu.Unlock()

		handle := gameWindow()
		if handle == 0 {
			continue
		}
		if enabled != appliedEnabled {
			if enabled {
				procImmAssociateContextEx.Call(
					uintptr(handle),
					0,
					iaceDefault,
				)
			} else {
				procImmAssociateContext.Call(uintptr(handle), 0)
			}
			appliedEnabled = enabled
			appliedCaret = image.Rectangle{}

			context, _, _ := procImmGetContext.Call(uintptr(handle))
			if context != 0 {
				procImmReleaseContext.Call(uintptr(handle), context)
			}
			imeState.mu.Lock()
			imeState.attached = context != 0
			imeState.mu.Unlock()
		}
		if enabled && caret != appliedCaret {
			setCompositionWindow(handle, caret)
			appliedCaret = caret
		}
	}
}

// setCompositionWindow places the composition and candidate windows at the
// caret. The caret rectangle is in logical pixels, the IME expects client
// pixels.
func setCompositionWindow(handle windows.HWND, caret image.Rectangle) {
	context, _, _ := procImmGetContext.Call(uintptr(handle))
	if context == 0 {
		return
	}
	defer procImmReleaseContext.Call(uintptr(handle), context)

	scale := ebiten.Monitor().DeviceScaleFactor()
	if scale <= 0 {
		scale = 1
	}
	form := win32CompositionForm{
		style: cfsPoint,
		currentPos: win32Point{
			x: int32(float64(caret.Min.X) * scale),
			y: int32(float64(caret.Min.Y) * scale),
		},
		area: win32Rect{
			left:   int32(float64(caret.Min.X) * scale),
			top:    int32(float64(caret.Min.Y) * scale),
			right:  int32(float64(caret.Max.X) * scale),
			bottom: int32(float64(caret.Max.Y) * scale),
		},
	}
	procImmSetCompositionWindow.Call(
		context,
		uintptr(unsafe.Pointer(&form)),
	)
}
