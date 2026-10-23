package frontend

import (
	"errors"
	"testing"
)

type recordingHostEventBackend struct {
	NullBackend
	events []HostEvent
}

func (b *recordingHostEventBackend) ReceiveHostEvent(event HostEvent) error {
	b.events = append(b.events, event)
	return nil
}

func TestShellCopiesAndForwardsNativeHostEvent(t *testing.T) {
	backend := &recordingHostEventBackend{}
	shell := &Shell{backend: backend}
	packet := []byte{1, 2, 3}
	if err := shell.ReceiveHostEvent(HostEvent{
		Kind: HostEventNetworkReceive,
		Data: packet,
	}); err != nil {
		t.Fatal(err)
	}
	packet[0] = 9
	if len(backend.events) != 1 || backend.events[0].Data[0] != 1 {
		t.Fatalf("forwarded events = %+v", backend.events)
	}
}

func TestShellRejectsInvalidOrUnsupportedHostEvent(t *testing.T) {
	backend := &recordingHostEventBackend{}
	shell := &Shell{backend: backend}
	if err := shell.ReceiveHostEvent(HostEvent{Kind: "unknown"}); err == nil {
		t.Fatal("unknown event kind was accepted")
	}
	if len(backend.events) != 0 {
		t.Fatal("invalid event reached backend")
	}
	shell.backend = NullBackend{}
	if err := shell.ReceiveHostEvent(HostEvent{Kind: HostEventSMSReceive}); !errors.Is(err, ErrHostEventUnsupported) {
		t.Fatalf("unsupported event error = %v", err)
	}
}
