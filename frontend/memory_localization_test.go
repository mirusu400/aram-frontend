package frontend

import (
	"errors"
	"strings"
	"testing"

	"github.com/ebitenui/ebitenui/widget"
)

func TestKoreanCatalogTranslatesMemorySearch(t *testing.T) {
	for _, message := range []string{
		"Memory Search", "Numeric type", "Comparison", "Search value", "Memory region",
		"Exact value", "Unknown initial value", "Increased", "Decreased", "Changed", "Unchanged",
		"Not equal", "Greater than", "Less than", "All searchable regions",
		"Decimal, 0x hex, or float", "First scan", "Next scan", "New search",
		"Choose a numeric type and region, then run a first scan.", "results", "Read-only region",
		"New value", "Enter a new numeric value", "Apply value", "Previous", "Refresh", "Next",
		"No matching addresses. Start a new search to try again.", "Refreshing memory session...",
		"Scan complete", "Search cleared", "Value changed; review the refreshed value before applying again",
	} {
		if got := translate(LanguageKorean, message); got == message || got == "" {
			t.Errorf("memory search message %q has no Korean translation", message)
		}
	}
}

func TestMemoryMessagesTranslateValuesAndJoinedErrors(t *testing.T) {
	shell := &Shell{}
	for _, test := range []struct{ message, korean string }{
		{"Wrote -12.5 to 0x03000000", "-12.5 값을 0x03000000 주소에 썼습니다"},
		{"heap (0x03000000, 33554432 bytes)", "heap (0x03000000, 33554432바이트)"},
		{"Scan complete: heap (0x03000000, 33554432 bytes)", "Scan complete: heap (0x03000000, 33554432바이트)"},
		{`"256" is outside the u8 range`, `"256"은(는) u8 범위를 벗어납니다`},
		{`"NaN" is not a finite f32 value`, `"NaN"은(는) 유한한 f32 값이 아닙니다`},
		{"Memory search is unavailable: open a supported game first", "메모리 검색을 사용할 수 없습니다: 지원되는 게임을 먼저 여세요"},
		{"The current result page is not readable: read scan result 0x03000000: synthetic failure", "현재 결과 페이지를 읽을 수 없습니다: 0x03000000 주소의 검색 결과 읽기 실패: synthetic failure"},
		{"Value changed; review the refreshed value before applying again\nmemory does not match expected original bytes", "값이 변경되었습니다. 갱신된 값을 확인한 뒤 다시 적용하세요\n메모리 값이 변경되었습니다. 갱신된 값을 확인하세요"},
		{`"a: 10%" is outside the u32 range`, `"a: 10%"은(는) u32 범위를 벗어납니다`},
		{"invalid next-scan comparison 0", "다음 검색에 사용할 수 없는 비교 방식: 0"},
		{"0x03000000 u32 100 heap", "0x03000000 u32 100 heap"},
	} {
		shell.settings.Language = string(LanguageKorean)
		if got := shell.trToolText(ToolMemory, test.message); got != test.korean {
			t.Errorf("Korean %q = %q, want %q", test.message, got, test.korean)
		}
		shell.settings.Language = string(LanguageEnglish)
		if got := shell.trToolText(ToolMemory, test.message); got != test.message {
			t.Errorf("English message changed: %q", got)
		}
	}
	shell.settings.Language = string(LanguageKorean)
	if got := shell.trToolText(ToolCheats, "Wrote 100 to 0x03000000"); got != "Wrote 100 to 0x03000000" {
		t.Fatalf("memory formatting changed another tool: %q", got)
	}
	for _, format := range memoryMessageFormats {
		if got := translate(LanguageKorean, format.message); got == format.message {
			t.Errorf("memory format %q has no Korean translation", format.message)
		}
	}
}

func TestMemoryPanelRendersKoreanControlsAndStatus(t *testing.T) {
	isolateSettledSettings(t)
	backend := newMemoryPanelBackend()
	backend.snapshot.Memory.Status = "Wrote 100 to 0x03000000"
	backend.snapshot.Fields[3].Options[0].Label = "heap (0x03000000, 33554432 bytes)"
	shell := NewShell(backend, nil, "")
	shell.settings.Language = string(LanguageKorean)
	shell.openToolPanel(ToolMemory)
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
	shell.interfaceUI.sync(shell)
	for field, want := range map[string]string{
		"type": "u32", "comparison": "정확한 값", "region": "heap (0x03000000, 33554432바이트)",
	} {
		if got := shell.interfaceUI.panelDropdowns[field].Label(); got != want {
			t.Errorf("%s dropdown = %q, want %q", field, got, want)
		}
	}
	if shell.panel.FieldValues["comparison"] != "equal" || shell.panel.FieldValues["region"] != "heap" {
		t.Fatal("localized controls changed backend field values")
	}
	form := widget.NewContainer()
	shell.interfaceUI.addMemoryResults(shell, shell.panel, form, nil)
	if got := form.Children()[0].(*widget.Text).Label; got != "100 값을 0x03000000 주소에 썼습니다" {
		t.Fatalf("rendered write status = %q", got)
	}
	shell.consumeToolResult(toolResult{
		kind: ToolMemory, snapshot: backend.snapshot,
		err:   errors.New(`"256" is outside the u8 range`),
		panel: shell.panel, generation: shell.frameGeneration, toolGeneration: shell.toolGeneration,
	})
	if !strings.Contains(shell.status, `"256"은(는) u8 범위를 벗어납니다`) {
		t.Fatalf("error status is not localized: %q", shell.status)
	}
	shell.panel.Memory = nil
	shell.panel.Lines = []string{"Memory search is unavailable: open a supported game first"}
	if got := shell.panelLines()[0]; got != "메모리 검색을 사용할 수 없습니다: 지원되는 게임을 먼저 여세요" {
		t.Fatalf("unavailable panel is not localized: %q", got)
	}
}
