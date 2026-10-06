package frontend

import (
	"context"
	"errors"
	"fmt"
	"image"
	"testing"
)

type memoryPanelBackend struct {
	NullBackend
	snapshot  ToolSnapshot
	requests  chan ToolRequest
	failWrite bool
}

func (b *memoryPanelBackend) ToolSnapshot(context.Context, ToolKind) (ToolSnapshot, error) {
	return b.snapshot, nil
}
func (b *memoryPanelBackend) ExecuteToolAction(_ context.Context, request ToolRequest) (ToolSnapshot, error) {
	b.requests <- request
	if request.Action == "select" {
		copy := *b.snapshot.Memory
		selected := copy.Results[0]
		copy.Selected = &selected
		b.snapshot.Memory = &copy
	}
	if request.Action == "write" && b.failWrite {
		copy := *b.snapshot.Memory
		selected := *copy.Selected
		selected.Value, selected.Expected = "101", "65"
		copy.Selected = &selected
		b.snapshot.Memory = &copy
		return b.snapshot, errors.New("value changed")
	}
	return b.snapshot, nil
}

func newMemoryPanelBackend() *memoryPanelBackend {
	return &memoryPanelBackend{
		requests: make(chan ToolRequest, 4),
		snapshot: ToolSnapshot{Title: "Memory Search", Session: 12,
			Fields: []ToolField{
				{ID: "type", Label: "Numeric type", Value: "u32", Options: []ToolFieldOption{{Value: "u32", Label: "u32"}}},
				{ID: "comparison", Label: "Comparison", Value: "equal", Options: []ToolFieldOption{{Value: "equal", Label: "Exact value"}}},
				{ID: "value", Label: "Search value", Value: "100"},
				{ID: "region", Label: "Memory region", Value: "heap", Options: []ToolFieldOption{{Value: "heap", Label: "heap"}}},
			},
			Actions: []ToolAction{{ID: "scan", Label: "First scan", Enabled: true}, {ID: "refine", Label: "Next scan", Enabled: true}, {ID: "reset", Label: "New search", Enabled: true}},
			Memory: &MemorySnapshot{Active: true, Type: "u32", Total: 8_388_608, PageSize: 32, Results: []MemoryResult{
				{Address: 0x03000000, Region: "heap", Value: "100", Type: "u32", Expected: "64", Writable: true},
			}},
		},
	}
}

func TestMemoryPanelSelectionWriteConflictAndInputCapture(t *testing.T) {
	isolateSettledSettings(t)
	backend := newMemoryPanelBackend()
	shell := NewShell(backend, nil, "")
	shell.openToolPanel(ToolMemory)
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
	if shell.guestInputAllowed() {
		t.Fatal("memory text entry reaches the guest")
	}
	shell.selectMemoryResult(shell.panel.Memory.Results[0])
	request := <-backend.requests
	if request.Action != "select" || request.Session != 12 || request.Fields["address"] != "0x03000000" {
		t.Fatalf("selection: %+v", request)
	}
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
	shell.panel.FieldValues["new_value"] = "200"
	backend.failWrite = true
	shell.writeSelectedMemory()
	request = <-backend.requests
	if request.Fields["expected"] != "64" || request.Fields["new_value"] != "200" {
		t.Fatalf("checked write: %+v", request)
	}
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
	if shell.panel.Memory.Selected.Value != "101" || shell.panel.Memory.Selected.Expected != "65" || shell.panel.Busy {
		t.Fatalf("conflict refresh: %+v", shell.panel)
	}
	if shell.panel.FieldValues["new_value"] != "200" {
		t.Fatal("conflict lost new-value edit")
	}
	shell.panel = nil
	if !shell.guestInputAllowed() {
		t.Fatal("closing search still captures guest input")
	}
	shell.openToolPanel(ToolMemory)
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
	if shell.panel.Memory.Total != 8_388_608 {
		t.Fatal("reopening discarded scan session")
	}
}

func TestMemoryPanelRejectsLateResponses(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(newMemoryPanelBackend(), nil, "")
	shell.openToolPanel(ToolMemory)
	stale := waitToolResult(t, shell.toolResults)
	shell.panel = nil
	shell.openToolPanel(ToolMemory)
	currentPanel := shell.panel
	shell.consumeToolResult(stale)
	if shell.panel != currentPanel || shell.panel.Memory != nil {
		t.Fatal("old request replaced reopened panel")
	}
	current := waitToolResult(t, shell.toolResults)
	shell.frameGeneration++ // A different loaded game invalidates the response.
	shell.consumeToolResult(current)
	if shell.panel == currentPanel || shell.panel.Memory != nil {
		t.Fatal("old game result populated new game")
	}
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
}

// The memory lifecycle generation must not swallow other tools' responses: a
// Cheats action in flight across Reset would otherwise leave the panel busy.
func TestLifecycleCommandKeepsOtherToolResponses(t *testing.T) {
	isolateSettledSettings(t)
	backend := newMemoryPanelBackend()
	backend.snapshot.Memory = nil
	shell := NewShell(backend, nil, "")
	shell.openToolPanel(ToolCheats)
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
	shell.executeToolAction("refresh", nil)
	<-backend.requests
	shell.executeBackend(CommandReset)
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
	if shell.panel == nil || shell.panel.Tool != ToolCheats || shell.panel.Busy || len(shell.panel.Actions) == 0 {
		t.Fatalf("cheats response dropped across reset: %+v", shell.panel)
	}
}

func TestMemoryPanelBuildsOnlyOnePageInResponsiveViewport(t *testing.T) {
	for _, size := range [][2]int{{390, 844}, {720, 540}, {960, 720}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			isolateSettledSettings(t)
			backend := newMemoryPanelBackend()
			for i := 1; i < 32; i++ {
				result := backend.snapshot.Memory.Results[0]
				result.Address += uint32(i * 4)
				backend.snapshot.Memory.Results = append(backend.snapshot.Memory.Results, result)
			}
			selected := backend.snapshot.Memory.Results[0]
			backend.snapshot.Memory.Selected = &selected
			shell := NewShell(backend, nil, "")
			shell.Layout(size[0], size[1])
			shell.openToolPanel(ToolMemory)
			shell.consumeToolResult(waitToolResult(t, shell.toolResults))
			view := shell.interfaceUI
			view.sync(shell)
			view.ui.Update()
			if len(view.memoryResultButtons) != 32 {
				t.Fatalf("rendered %d result widgets", len(view.memoryResultButtons))
			}
			if view.memoryScroll == nil || view.panelTextInputs["new_value"] == nil {
				t.Fatal("missing result scrolling or value editor")
			}
			rect := view.panelWindow.GetContainer().GetWidget().Rect
			if !rect.In(image.Rect(0, 0, size[0], size[1])) {
				t.Fatalf("memory window outside viewport: %v", rect)
			}
			if !view.memoryScroll.ViewRect().In(rect) {
				t.Fatalf("scroll area outside window: %v", view.memoryScroll.ViewRect())
			}
		})
	}
}
