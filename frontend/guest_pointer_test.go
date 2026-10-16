package frontend

import (
	"errors"
	"image"
	"testing"
)

type pointerRecorder struct {
	NullBackend
	events []PointerInputEvent
	err    error
}

func (r *pointerRecorder) SupportsPointer() bool { return true }
func (r *pointerRecorder) QueuePointer(e PointerInputEvent) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, e)
	return nil
}

func TestGuestPointerReleaseRetriesUntilAccepted(t *testing.T) {
	backend := &pointerRecorder{err: errors.New("queue full")}
	shell := &Shell{guestPointer: guestPointerCapture{active: true, point: image.Pt(31, 47)}}
	if shell.finishGuestPointer(backend) || !shell.guestPointer.active {
		t.Fatal("failed release discarded capture")
	}
	backend.err = nil
	if !shell.finishGuestPointer(backend) || shell.guestPointer.active {
		t.Fatal("accepted release retained capture")
	}
	if len(backend.events) != 1 || backend.events[0] != (PointerInputEvent{X: 31, Y: 47}) {
		t.Fatalf("release=%v", backend.events)
	}
	if !shell.inputFramePending {
		t.Fatal("release did not wake execution")
	}
}

func TestGuestPointerMappingInvertsActualDrawTransform(t *testing.T) {
	for _, rotation := range []int{0, 90, 180, 270} {
		m := guestPointerMapping{source: image.Rect(0, 0, 240, 376),
			destination: image.Rect(20, 40, 500, 792), rotation: rotation}
		options := textureDrawOptions(m.source, m.destination, rotation, textureFilterNearest)
		for _, point := range []image.Point{{0, 0}, {31, 47}, {239, 375}, {120, 188}} {
			x, y := options.GeoM.Apply(float64(point.X)+0.5, float64(point.Y)+0.5)
			got, ok := m.guestPoint(image.Pt(int(x), int(y)), false)
			if !ok || got != point {
				t.Fatalf("rotation %d mapped %v to %v, valid=%t", rotation, point, got, ok)
			}
		}
	}
}

func TestGuestPointerMappingMatchesDisplayRotation(t *testing.T) {
	for _, test := range []struct {
		rotation int
		want     image.Point
	}{
		{0, image.Pt(0, 0)}, {90, image.Pt(0, 2)},
		{180, image.Pt(3, 2)}, {270, image.Pt(3, 0)},
	} {
		mapping := guestPointerMapping{source: image.Rect(7, 8, 11, 11),
			destination: image.Rect(10, 20, 50, 50), rotation: test.rotation}
		point, ok := mapping.guestPoint(image.Pt(10, 20), false)
		if !ok || point != test.want {
			t.Fatalf("rotation %d point=%v, want %v", test.rotation, point, test.want)
		}
		if _, ok := mapping.guestPoint(image.Pt(9, 21), false); ok {
			t.Fatal("letterbox press accepted")
		}
		point, ok = mapping.guestPoint(image.Pt(100, 100), true)
		if !ok || !point.In(image.Rect(0, 0, 4, 3)) {
			t.Fatalf("clamped release=%v", point)
		}
	}
}

func TestGuestPointerMappingUsesScaledInterior(t *testing.T) {
	mapping := guestPointerMapping{source: image.Rect(0, 0, 240, 376),
		destination: image.Rect(20, 40, 500, 792)}
	point, ok := mapping.guestPoint(image.Pt(82, 134), false)
	if !ok || point != image.Pt(31, 47) {
		t.Fatalf("point=%v, accepted=%v", point, ok)
	}
	if _, ok := (guestPointerMapping{}).guestPoint(image.Point{}, true); ok {
		t.Fatal("empty frame accepted pointer")
	}
}
