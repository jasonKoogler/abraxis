package grpc

import (
	"fmt"
	"sync"
	"sync/atomic"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
)

// AuthEventBus is a channel-based fan-out event bus for auth data events.
// Subscribers receive events on a buffered channel. If a subscriber's channel
// is full, the event is dropped for that subscriber (non-blocking publish).
type AuthEventBus struct {
	mu          sync.RWMutex
	subscribers map[string]chan *pb.AuthDataEvent
	version     atomic.Int64
}

func NewAuthEventBus() *AuthEventBus {
	return &AuthEventBus{
		subscribers: make(map[string]chan *pb.AuthDataEvent),
	}
}

func (b *AuthEventBus) Subscribe(id string) <-chan *pb.AuthDataEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan *pb.AuthDataEvent, 64)
	b.subscribers[id] = ch
	return ch
}

func (b *AuthEventBus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}
}

func (b *AuthEventBus) Publish(event *pb.AuthDataEvent) {
	ver := b.version.Add(1)
	event.Version = formatVersion(ver)
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *AuthEventBus) NextVersion() string {
	return formatVersion(b.version.Load())
}

func formatVersion(v int64) string {
	return fmt.Sprintf("%d", v)
}

// PolicyEventBus is a channel-based fan-out event bus for policy events.
type PolicyEventBus struct {
	mu          sync.RWMutex
	subscribers map[string]chan *pb.PolicyEvent
	version     atomic.Int64
}

func NewPolicyEventBus() *PolicyEventBus {
	return &PolicyEventBus{
		subscribers: make(map[string]chan *pb.PolicyEvent),
	}
}

func (b *PolicyEventBus) Subscribe(id string) <-chan *pb.PolicyEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan *pb.PolicyEvent, 16)
	b.subscribers[id] = ch
	return ch
}

func (b *PolicyEventBus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}
}

func (b *PolicyEventBus) Publish(event *pb.PolicyEvent) {
	ver := b.version.Add(1)
	event.Version = formatVersion(ver)
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *PolicyEventBus) NextVersion() string {
	return formatVersion(b.version.Load())
}
