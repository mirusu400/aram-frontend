package frontend

// ReceiveHostEvent is the thread-safe native-host entry point for network,
// serial, and SMS notifications. The backend owns guest execution
// serialization; the shell copies packet data so callers may reuse their
// buffer as soon as this method returns.
func (s *Shell) ReceiveHostEvent(event HostEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	receiver, ok := s.backend.(HostEventReceiver)
	if !ok {
		return ErrHostEventUnsupported
	}
	event.Data = append([]byte(nil), event.Data...)
	return receiver.ReceiveHostEvent(event)
}
