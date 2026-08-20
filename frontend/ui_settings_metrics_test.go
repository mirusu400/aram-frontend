package frontend

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var settingsSectionsUnderTest = []string{
	"General", "Appearance", "Graphics", "Audio", "Controls", "Bindings", "Updates",
}

// TestSettingsRowGeometryFitsPanel is the regression guard for a settings
// panel whose action column walked off screen. The rows are stretched to the
// panel width, so label, description, and control have to be budgeted from
// that width; when the copy was free to report whatever width it liked, the
// pixel type ramp widened the scroll content past the panel and took every
// right-anchored control with it.
func TestSettingsRowGeometryFitsPanel(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := NewShell(NullBackend{}, nil, "")

	for _, family := range themeFamilyChoices() {
		shell.settings.ThemeFamily = family
		shell.syncDesignSystem()
		design := shell.design
		u := shell.interfaceUI
		// Phone-shaped viewports matter as much as desktop ones: the mobile
		// build reports the device's pixel size, which is narrow and tall.
		for _, viewport := range [][2]int{
			{976, 759}, {1280, 900}, {640, 480},
			{1080, 2280}, {720, 1440}, {390, 844},
		} {
			u.viewportWidth, u.viewportHeight = viewport[0], viewport[1]
			u.compact = viewport[0] < 820 || viewport[1] < 620
			rowsWidth := u.settingsRowsWidth(design)
			for _, section := range settingsSectionsUnderTest {
				u.settingsSection = section
				models := u.settingsRowModels(shell)
				actionWidth := u.settingsActionWidth(shell, models)
				floor := settingsActionMinWidth
				if u.compact {
					floor = settingsCompactActionMinWidth
				}
				if actionWidth < floor {
					t.Errorf("%s/%dx%d/%s: action column %dpx is below the %dpx floor",
						family, viewport[0], viewport[1], section, actionWidth, floor)
				}
				copyWidth := u.settingsCopyWidth(design, actionWidth)
				// A stacked row spends its width on one thing at a time; a
				// side-by-side row has to fit the copy and the control at once.
				budget := copyWidth + 2*design.Space.M
				if !u.settingsRowStacks(design, actionWidth) {
					budget = design.Space.M + copyWidth + design.Space.L + actionWidth
				}
				if budget > rowsWidth {
					t.Errorf("%s/%dx%d/%s: budget %dpx exceeds the %dpx row",
						family, viewport[0], viewport[1], section, budget, rowsWidth)
				}
				for _, model := range models {
					// AnchorLayout sizes a row from its first child alone, so
					// the copy block is what has to stay inside the budget:
					// text wider than that runs over the control anchored to
					// the row's far edge, which is how the buttons became
					// unreachable.
					row := u.buildSettingsRow(model, actionWidth)
					children := row.Children()
					if len(children) == 0 {
						t.Fatalf("%s/%s/%q: row has no children",
							family, section, model.label)
					}
					width, _ := children[0].PreferredSize()
					if width > copyWidth {
						t.Errorf("%s/%dx%d/%s/%q: label and description want"+
							" %dpx of the %dpx left beside a %dpx control",
							family, viewport[0], viewport[1], section,
							model.label, width, copyWidth, actionWidth)
					}
				}
			}
		}
	}
}

// TestSettingsActionColumnFitsItsLabels checks the column is measured from the
// labels it has to hold, not from a constant that only suited one type ramp.
func TestSettingsActionColumnFitsItsLabels(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := NewShell(NullBackend{}, nil, "")
	shell.settings.ThemeFamily = "chrome-blue"
	shell.syncDesignSystem()
	design := shell.design
	u := shell.interfaceUI
	u.viewportWidth, u.viewportHeight = 976, 759

	for _, section := range settingsSectionsUnderTest {
		u.settingsSection = section
		models := u.settingsRowModels(shell)
		actionWidth := u.settingsActionWidth(shell, models)
		labelWidth := actionWidth - 2*design.Space.S
		for _, model := range models {
			if model.slider != nil {
				continue
			}
			labels := []string{shell.tr(model.value)}
			if model.dropdown != nil {
				labels = labels[:0]
				for index := 0; index < model.dropdown.count; index++ {
					labels = append(labels, model.dropdown.label(index))
				}
			}
			for _, label := range labels {
				fitted := fitTextToWidth(label, design.Type.Strong, labelWidth)
				width, _ := text.Measure(fitted, *design.Type.Strong, 0)
				if int(width) > labelWidth {
					t.Errorf("%s/%q: label renders %dpx wide in a %dpx column",
						section, label, int(width), labelWidth)
				}
			}
		}
	}
}

// TestTypeRampScaleGrowsTheDialog pins the dialog growing with the ramp rather
// than staying at the size the modern faces needed.
func TestTypeRampScaleGrowsTheDialog(t *testing.T) {
	modern := newARAMDesignSystem("dark", themeFamilyModern)
	retro := newARAMDesignSystem("dark", "chrome-blue")
	modernWidth, modernHeight := settingsWindowSize(modern)
	retroWidth, retroHeight := settingsWindowSize(retro)
	if modernWidth != settingsBaseWindowWidth || modernHeight != settingsBaseWindowHeight {
		t.Errorf("modern dialog = %dx%d, want the base %dx%d",
			modernWidth, modernHeight, settingsBaseWindowWidth, settingsBaseWindowHeight)
	}
	if retroWidth <= modernWidth || retroHeight <= modernHeight {
		t.Errorf("pixel-ramp dialog = %dx%d, want more than the modern %dx%d",
			retroWidth, retroHeight, modernWidth, modernHeight)
	}
}
