package hostclient

import (
	"fmt"
	"sync"
)

// Pool manages gRPC connections to all Host Agents.
// Reuses connections — one per host.
type Pool struct {
	mu      sync.RWMutex
	clients map[string]*Client // hostID → client
}

func NewPool() *Pool {
	return &Pool{clients: make(map[string]*Client)}
}

// Get returns a client for the given host, creating one if needed.
func (p *Pool) Get(hostID, address string, grpcPort int) (*Client, error) {
	p.mu.RLock()
	if c, ok := p.clients[hostID]; ok {
		p.mu.RUnlock()
		return c, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check after acquiring write lock
	if c, ok := p.clients[hostID]; ok {
		return c, nil
	}

	addr := fmt.Sprintf("%s:%d", address, grpcPort)
	c, err := Dial(hostID, addr)
	if err != nil {
		return nil, err
	}

	p.clients[hostID] = c
	return c, nil
}

// CloseAll closes all connections.
func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.clients {
		c.Close()
	}
	p.clients = make(map[string]*Client)
}
