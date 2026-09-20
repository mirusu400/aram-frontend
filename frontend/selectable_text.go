package frontend

import (
	"image"
	"math"
	"strings"

	"github.com/ebitenui/ebitenui/input"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// selectableText is a read-only, multi-line text surface. It deliberately
// owns selection rather than editing so diagnostic text can be copied without
// an accidental key press changing what the application reported.
type selectableText struct {
	design *ARAMDesignSystem
	face   *text.Face
	value  string
	lines  []string
	starts []int

	widget     *widget.Widget
	padding    widget.Insets
	lineHeight int
	width      int
	height     int

	anchor   int
	caret    int
	dragging bool
	focused  bool
}

func newSelectableText(
	design *ARAMDesignSystem,
	value string,
	width int,
	height int,
	layoutData interface{},
) *selectableText {
	face := design.Type.Body
	_, measuredHeight := text.Measure(" ", *face, 0)
	view := &selectableText{
		design:     design,
		face:       face,
		value:      value,
		lines:      strings.Split(value, "\n"),
		padding:    widget.Insets{Left: design.Space.S, Top: design.Space.S, Right: design.Space.S, Bottom: design.Space.S},
		lineHeight: int(math.Ceil(measuredHeight)) + design.px(2),
		width:      width,
		height:     height,
	}
	view.starts = lineStartOffsets(view.lines)
	view.widget = widget.NewWidget(
		widget.WidgetOpts.TrackHover(true),
		widget.WidgetOpts.MinSize(width, height),
		widget.WidgetOpts.LayoutData(layoutData),
	)
	return view
}

func lineStartOffsets(lines []string) []int {
	starts := make([]int, len(lines))
	offset := 0
	for index, line := range lines {
		starts[index] = offset
		offset += len(line)
		if index+1 < len(lines) {
			offset++
		}
	}
	return starts
}

func (t *selectableText) GetWidget() *widget.Widget { return t.widget }

func (t *selectableText) SetLocation(rect image.Rectangle) { t.widget.Rect = rect }

func (t *selectableText) PreferredSize() (int, int) { return t.width, t.height }

func (t *selectableText) Validate() {}

func (t *selectableText) Update(updObj *widget.UpdateObject) {
	t.widget.Update(updObj)
	t.updatePointer()
	if !t.focused || !shortcutModifierPressed() {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyA) {
		t.anchor, t.caret = 0, len(t.value)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		if selected := t.selectedText(); selected != "" {
			_ = writeClipboardText(selected)
		}
	}
}

func (t *selectableText) updatePointer() {
	if input.MouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := input.CursorPosition()
		inside := image.Pt(x, y).In(t.widget.Rect) &&
			input.MouseButtonJustPressedLayer(
				ebiten.MouseButtonLeft,
				t.widget.EffectiveInputLayer(),
			)
		if inside {
			t.focused = true
			t.anchor = t.byteIndexAtCursor(x, y)
			t.caret = t.anchor
			t.dragging = true
		} else {
			t.focused = false
			t.dragging = false
		}
	}
	if !t.dragging {
		return
	}
	if !input.MouseButtonPressed(ebiten.MouseButtonLeft) {
		t.dragging = false
		return
	}
	x, y := input.CursorPosition()
	t.caret = t.byteIndexAtCursor(x, y)
}

func (t *selectableText) byteIndexAtCursor(x, y int) int {
	if len(t.lines) == 0 {
		return 0
	}
	lineIndex := (y - t.widget.Rect.Min.Y - t.padding.Top) / t.lineHeight
	lineIndex = max(0, min(len(t.lines)-1, lineIndex))
	line := t.lines[lineIndex]
	offset := x - t.widget.Rect.Min.X - t.padding.Left
	return t.starts[lineIndex] + byteIndexAtAdvance(line, t.face, float64(offset))
}

func (t *selectableText) selection() (int, int) {
	if t.anchor <= t.caret {
		return t.anchor, t.caret
	}
	return t.caret, t.anchor
}

func (t *selectableText) selectedText() string {
	start, end := t.selection()
	if start == end {
		return ""
	}
	return t.value[start:end]
}

func shortcutModifierPressed() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControl) ||
		ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyControlRight) ||
		ebiten.IsKeyPressed(ebiten.KeyMeta) ||
		ebiten.IsKeyPressed(ebiten.KeyMetaLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyMetaRight)
}

func (t *selectableText) Render(screen *ebiten.Image) {
	t.widget.Render(screen)
	rect := t.widget.Rect
	t.design.Components.ControlGroup.Draw(
		screen,
		rect.Dx(),
		rect.Dy(),
		func(options *ebiten.DrawImageOptions) {
			options.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
		},
	)
	inner := image.Rect(
		rect.Min.X+t.padding.Left,
		rect.Min.Y+t.padding.Top,
		rect.Max.X-t.padding.Right,
		rect.Max.Y-t.padding.Bottom,
	).Intersect(screen.Bounds())
	if inner.Empty() {
		return
	}
	clip, ok := screen.SubImage(inner).(*ebiten.Image)
	if !ok {
		return
	}
	selectionStart, selectionEnd := t.selection()
	for lineIndex, line := range t.lines {
		top := inner.Min.Y + lineIndex*t.lineHeight
		if top+t.lineHeight > inner.Max.Y {
			break
		}
		lineStart := t.starts[lineIndex]
		lineEnd := lineStart + len(line)
		start := max(selectionStart, lineStart)
		end := min(selectionEnd, lineEnd)
		if start < end {
			from := advanceWidth(line[:start-lineStart], t.face)
			to := advanceWidth(line[:end-lineStart], t.face)
			vector.DrawFilledRect(
				clip,
				float32(inner.Min.X+from),
				float32(top),
				float32(to-from),
				float32(t.lineHeight),
				t.design.Palette.AccentSoft,
				false,
			)
		}
		options := &text.DrawOptions{}
		options.GeoM.Translate(float64(inner.Min.X), float64(top))
		options.ColorScale.ScaleWithColor(t.design.Palette.TextMuted)
		text.Draw(clip, line, *t.face, options)
	}
}
