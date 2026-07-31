package frontend

import (
	"image"
	"math"
	"unicode/utf8"

	"github.com/ebitenui/ebitenui/input"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// EbitenUI's TextInput reads committed characters through
// ebiten.AppendInputChars only. Ebitengine keeps the IME detached from the
// window until an exp/textinput session is started, so composed scripts such
// as Hangul never reach that widget. imeTextInput is a single line field that
// owns an exp/textinput.Field instead, which starts the session, renders the
// composition in place, and falls back to AppendInputChars on platforms
// without an IME backend.
type imeTextInput struct {
	field textinput.Field

	design      *ARAMDesignSystem
	face        *text.Face
	placeholder string
	changed     func(string)

	widget     *widget.Widget
	padding    widget.Insets
	lineHeight int

	caret    int
	anchor   int
	dragging bool
	focused  bool

	scrollOffset int
	caretOffset  int
	blink        int
	lastText     string

	tabOrder int
	focusMap map[widget.FocusDirection]widget.Focuser
}

type imeTextInputConfig struct {
	Placeholder string
	Text        string
	Disabled    bool
	MinHeight   int
	LayoutData  interface{}
	Changed     func(string)
}

const (
	imeCaretWidth    = 2
	imeBlinkInterval = 30
	imeRepeatDelay   = 30
	imeRepeatPeriod  = 3
)

func newIMETextInput(
	design *ARAMDesignSystem,
	config imeTextInputConfig,
) *imeTextInput {
	face := design.Type.Body
	_, height := text.Measure(" ", *face, 0)
	field := &imeTextInput{
		design:      design,
		face:        face,
		placeholder: config.Placeholder,
		changed:     config.Changed,
		padding:     widget.Insets{Left: design.Space.S, Right: design.Space.S},
		lineHeight:  int(math.Ceil(height)),
		focusMap:    make(map[widget.FocusDirection]widget.Focuser),
	}
	minHeight := config.MinHeight
	if minHeight <= 0 {
		minHeight = field.lineHeight + design.Space.S*2
	}
	options := []widget.WidgetOpt{
		widget.WidgetOpts.TrackHover(true),
		widget.WidgetOpts.MinSize(0, minHeight),
	}
	if config.LayoutData != nil {
		options = append(
			options,
			widget.WidgetOpts.LayoutData(config.LayoutData),
		)
	}
	field.widget = widget.NewWidget(options...)
	field.widget.Disabled = config.Disabled
	field.SetText(config.Text)
	return field
}

/** PreferredSizeLocateableWidget interface **/

func (t *imeTextInput) GetWidget() *widget.Widget {
	return t.widget
}

func (t *imeTextInput) SetLocation(rect image.Rectangle) {
	t.widget.Rect = rect
}

func (t *imeTextInput) PreferredSize() (int, int) {
	width, height := 50, t.lineHeight+t.design.Space.S*2
	if height < t.widget.MinHeight {
		height = t.widget.MinHeight
	}
	if width < t.widget.MinWidth {
		width = t.widget.MinWidth
	}
	return width, height
}

func (t *imeTextInput) Validate() {}

/** Focuser interface **/

func (t *imeTextInput) Focus(focused bool) {
	if focused == t.focused {
		return
	}
	t.focused = focused
	t.blink = 0
	if focused {
		t.field.Focus()
	} else {
		t.field.Blur()
		t.dragging = false
	}
	t.widget.FireFocusEvent(t, focused, image.Point{-1, -1})
}

func (t *imeTextInput) IsFocused() bool {
	return t.focused
}

func (t *imeTextInput) TabOrder() int {
	return t.tabOrder
}

func (t *imeTextInput) GetFocus(
	direction widget.FocusDirection,
) widget.Focuser {
	return t.focusMap[direction]
}

func (t *imeTextInput) AddFocus(
	direction widget.FocusDirection,
	focus widget.Focuser,
) {
	t.focusMap[direction] = focus
}

/** Text access **/

func (t *imeTextInput) GetText() string {
	return t.field.Text()
}

func (t *imeTextInput) SetText(value string) {
	t.field.SetTextAndSelection(value, len(value), len(value))
	t.caret = len(value)
	t.anchor = t.caret
	t.lastText = value
}

/** Update **/

func (t *imeTextInput) Update(updObj *widget.UpdateObject) {
	t.widget.Update(updObj)
	if t.focused && !t.field.IsFocused() {
		// Another field took the single global text input session.
		t.focused = false
		t.dragging = false
	}
	if t.widget.Disabled {
		t.Focus(false)
		return
	}
	t.updatePointer()
	if t.focused && !t.updateComposition() {
		t.updateCommands()
	}
	t.notifyChanged()
	t.blink++
}

func (t *imeTextInput) updatePointer() {
	if input.MouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := input.CursorPosition()
		inside := image.Pt(x, y).In(t.widget.Rect) &&
			input.MouseButtonJustPressedLayer(
				ebiten.MouseButtonLeft,
				t.widget.EffectiveInputLayer(),
			)
		switch {
		case inside:
			t.Focus(true)
			index := t.byteIndexAtCursor(x)
			t.caret, t.anchor = index, index
			t.dragging = true
			t.blink = 0
		case t.focused:
			t.Focus(false)
		}
	}
	if !t.dragging {
		return
	}
	if !input.MouseButtonPressed(ebiten.MouseButtonLeft) {
		t.dragging = false
		return
	}
	x, _ := input.CursorPosition()
	t.caret = t.byteIndexAtCursor(x)
	t.blink = 0
}

func (t *imeTextInput) byteIndexAtCursor(x int) int {
	offset := x - t.widget.Rect.Min.X - t.padding.Left - t.scrollOffset
	return byteIndexAtAdvance(t.field.Text(), t.face, float64(offset))
}

// updateComposition feeds the tick to the text input session and reports
// whether the IME consumed it. A consumed tick must not be interpreted as an
// editing command as well, otherwise the backspace that shortens a Hangul
// composition would also delete an already committed character.
func (t *imeTextInput) updateComposition() bool {
	start, end := t.selection()
	if currentStart, currentEnd := t.field.Selection(); currentStart != start ||
		currentEnd != end {
		t.field.SetSelection(start, end)
	}
	handled, err := t.field.HandleInputWithBounds(t.caretBounds())
	if err != nil {
		return false
	}
	currentStart, currentEnd := t.field.Selection()
	if currentStart != start || currentEnd != end {
		t.anchor, t.caret = currentStart, currentEnd
		t.blink = 0
	}
	return handled || t.field.UncommittedTextLengthInBytes() > 0
}

func (t *imeTextInput) updateCommands() {
	value := t.field.Text()
	extend := ebiten.IsKeyPressed(ebiten.KeyShift)
	if ebiten.IsKeyPressed(ebiten.KeyControl) {
		if inpututil.IsKeyJustPressed(ebiten.KeyA) {
			t.anchor, t.caret = 0, len(value)
			t.blink = 0
		}
		return
	}
	switch {
	case imeKeyTriggered(ebiten.KeyLeft):
		t.moveCaret(previousRuneBoundary(value, t.caret), extend)
	case imeKeyTriggered(ebiten.KeyRight):
		t.moveCaret(nextRuneBoundary(value, t.caret), extend)
	case imeKeyTriggered(ebiten.KeyHome):
		t.moveCaret(0, extend)
	case imeKeyTriggered(ebiten.KeyEnd):
		t.moveCaret(len(value), extend)
	case imeKeyTriggered(ebiten.KeyBackspace):
		t.deleteBackward()
	case imeKeyTriggered(ebiten.KeyDelete):
		t.deleteForward()
	}
}

func imeKeyTriggered(key ebiten.Key) bool {
	pressed := inpututil.KeyPressDuration(key)
	if pressed == 1 {
		return true
	}
	return pressed > imeRepeatDelay &&
		(pressed-imeRepeatDelay)%imeRepeatPeriod == 0
}

func (t *imeTextInput) moveCaret(index int, extend bool) {
	t.caret = index
	if !extend {
		t.anchor = index
	}
	t.blink = 0
}

func (t *imeTextInput) deleteBackward() {
	start, end := t.selection()
	if start == end {
		if start == 0 {
			return
		}
		t.anchor = previousRuneBoundary(t.field.Text(), start)
		t.caret = start
	}
	t.replaceSelection("")
}

func (t *imeTextInput) deleteForward() {
	value := t.field.Text()
	start, end := t.selection()
	if start == end {
		if end == len(value) {
			return
		}
		t.anchor = start
		t.caret = nextRuneBoundary(value, start)
	}
	t.replaceSelection("")
}

func (t *imeTextInput) replaceSelection(replacement string) {
	value := t.field.Text()
	start, end := t.selection()
	next := value[:start] + replacement + value[end:]
	position := start + len(replacement)
	t.field.SetTextAndSelection(next, position, position)
	t.caret, t.anchor = position, position
	t.blink = 0
}

func (t *imeTextInput) selection() (int, int) {
	if t.anchor <= t.caret {
		return t.anchor, t.caret
	}
	return t.caret, t.anchor
}

func (t *imeTextInput) notifyChanged() {
	value := t.field.Text()
	if value == t.lastText {
		return
	}
	t.lastText = value
	if t.changed != nil {
		t.changed(value)
	}
}

func (t *imeTextInput) caretBounds() image.Rectangle {
	x := t.widget.Rect.Min.X + t.padding.Left + t.scrollOffset + t.caretOffset
	y := t.textTop()
	return image.Rect(x, y, x+1, y+t.lineHeight)
}

func (t *imeTextInput) textTop() int {
	return t.widget.Rect.Min.Y + (t.widget.Rect.Dy()-t.lineHeight)/2
}

/** Render **/

func (t *imeTextInput) Render(screen *ebiten.Image) {
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
	t.renderContent(screen)
}

func (t *imeTextInput) renderContent(screen *ebiten.Image) {
	inner := image.Rect(
		t.widget.Rect.Min.X+t.padding.Left,
		t.widget.Rect.Min.Y,
		t.widget.Rect.Max.X-t.padding.Right,
		t.widget.Rect.Max.Y,
	).Intersect(screen.Bounds())
	if inner.Empty() {
		return
	}
	clip, ok := screen.SubImage(inner).(*ebiten.Image)
	if !ok {
		return
	}

	palette := t.design.Palette
	display := t.field.TextForRendering()
	composing := t.field.UncommittedTextLengthInBytes()
	compositionStart, _ := t.field.Selection()
	caretIndex := t.caret
	if composing > 0 {
		caretIndex = compositionStart + composing
		if _, cursor, ok := t.field.CompositionSelection(); ok {
			caretIndex = compositionStart + cursor
		}
	}
	caretOffset := advanceWidth(display[:min(caretIndex, len(display))], t.face)
	t.caretOffset = caretOffset
	t.updateScroll(caretOffset, inner.Dx())

	top := t.textTop()
	left := inner.Min.X + t.scrollOffset

	if start, end := t.selection(); composing == 0 && start != end && t.focused {
		from := advanceWidth(display[:start], t.face)
		to := advanceWidth(display[:end], t.face)
		vector.DrawFilledRect(
			clip,
			float32(left+from),
			float32(top),
			float32(to-from),
			float32(t.lineHeight),
			palette.AccentSoft,
			false,
		)
	}

	label, labelColor := display, palette.Text
	if t.widget.Disabled {
		labelColor = palette.TextDisabled
	}
	if label == "" {
		label, labelColor = t.placeholder, palette.TextDisabled
	}
	options := &text.DrawOptions{}
	options.GeoM.Translate(float64(left), float64(top))
	options.ColorScale.ScaleWithColor(labelColor)
	text.Draw(clip, label, *t.face, options)

	if composing > 0 {
		from := advanceWidth(display[:compositionStart], t.face)
		to := advanceWidth(display[:compositionStart+composing], t.face)
		vector.DrawFilledRect(
			clip,
			float32(left+from),
			float32(top+t.lineHeight-1),
			float32(to-from),
			1,
			palette.Accent,
			false,
		)
	}

	if t.focused && t.blink%(imeBlinkInterval*2) < imeBlinkInterval {
		caretColor := palette.Accent
		if t.widget.Disabled {
			caretColor = palette.TextDisabled
		}
		vector.DrawFilledRect(
			clip,
			float32(left+caretOffset),
			float32(top),
			imeCaretWidth,
			float32(t.lineHeight),
			caretColor,
			false,
		)
	}
}

func (t *imeTextInput) updateScroll(caretOffset int, visibleWidth int) {
	if visibleWidth <= 0 {
		return
	}
	if caretOffset+t.scrollOffset+imeCaretWidth > visibleWidth {
		t.scrollOffset = visibleWidth - caretOffset - imeCaretWidth
	}
	if caretOffset+t.scrollOffset < 0 {
		t.scrollOffset = -caretOffset
	}
	if t.scrollOffset > 0 {
		t.scrollOffset = 0
	}
}

/** Text measuring helpers **/

func advanceWidth(value string, face *text.Face) int {
	if value == "" {
		return 0
	}
	return int(math.Round(text.Advance(value, *face)))
}

// runeBoundaries lists every byte offset a caret may occupy, including the
// empty prefix and the end of the string.
func runeBoundaries(value string) []int {
	boundaries := make([]int, 0, utf8.RuneCountInString(value)+1)
	for index := range value {
		boundaries = append(boundaries, index)
	}
	return append(boundaries, len(value))
}

func previousRuneBoundary(value string, index int) int {
	if index <= 0 {
		return 0
	}
	if index > len(value) {
		index = len(value)
	}
	_, size := utf8.DecodeLastRuneInString(value[:index])
	return index - size
}

func nextRuneBoundary(value string, index int) int {
	if index >= len(value) {
		return len(value)
	}
	if index < 0 {
		index = 0
	}
	_, size := utf8.DecodeRuneInString(value[index:])
	return index + size
}

// byteIndexAtAdvance maps a pixel offset inside the rendered text to the
// nearest caret position. Prefix advances grow monotonically, so the boundary
// is found with a binary search instead of measuring every prefix.
func byteIndexAtAdvance(value string, face *text.Face, offset float64) int {
	if value == "" || offset <= 0 {
		return 0
	}
	boundaries := runeBoundaries(value)
	low, high := 0, len(boundaries)-1
	for low < high {
		middle := (low + high + 1) / 2
		if text.Advance(value[:boundaries[middle]], *face) <= offset {
			low = middle
		} else {
			high = middle - 1
		}
	}
	if low+1 >= len(boundaries) {
		return boundaries[low]
	}
	before := text.Advance(value[:boundaries[low]], *face)
	after := text.Advance(value[:boundaries[low+1]], *face)
	if offset-before > after-offset {
		return boundaries[low+1]
	}
	return boundaries[low]
}
