package main

import (
	"fmt"
	"sync"
	"time"
)

// RingRegistry holds all active rings and a fast node lookup index.
type RingRegistry struct {
	rings     map[string]*Ring
	nodeIndex map[string]*Node
	mu        sync.RWMutex
}

// Ring is a directed group of nodes: A→B, B→C, C→A
type Ring struct {
	ID    string
	Nodes []*Node
}

// Node represents a single peer in the network.
type Node struct {
	ID        string
	IP        string
	NATType   string
	Score     NodeScore
	RingID    string
	NextHop   string // the node ID this node proxies traffic for
	WarmSpare string // backup proxy if NextHop goes offline
	LastSeen  time.Time
}

// NodeScore tracks the quality metrics used to rank nodes.
type NodeScore struct {
	Uptime    float64
	Bandwidth float64
	Latency   float64
	Composite float64
}

// Session tracks an active client connection through the network.
type Session struct {
	ClientID   string
	RingID     string
	ProxyID    string
	SpareID    string
	CreatedAt  time.Time
	LastActive time.Time
}

// ProxyAssignment is what the broker returns to a client requesting a proxy.
type ProxyAssignment struct {
	RingID       string `json:"ring_id"`
	ProxyID      string `json:"proxy_id"`
	ProxyIP      string `json:"proxy_ip"`
	SpareID      string `json:"spare_id"`
	SpareIP      string `json:"spare_ip"`
	SessionToken string `json:"session_token"`
}

// NewRingRegistry initializes an empty registry.
func NewRingRegistry() *RingRegistry {
	return &RingRegistry{
		rings:     make(map[string]*Ring),
		nodeIndex: make(map[string]*Node),
	}
}

// AssignProxy finds an available node and assigns it to the requesting client.
func (r *RingRegistry) AssignProxy(clientID string, natType string, premium bool) (*ProxyAssignment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the first available node that isn't the client itself
	for _, node := range r.nodeIndex {
		if node.ID != clientID {
			return &ProxyAssignment{
				RingID:       "ring-001",
				ProxyID:      node.ID,
				ProxyIP:      node.IP,
				SpareID:      "none",
				SpareIP:      "none",
				SessionToken: "token-" + clientID,
			}, nil
		}
	}

	return nil, fmt.Errorf("no available nodes for client %s", clientID)
}

// RegisterNode adds a new node to the registry or updates an existing one.
func (r *RingRegistry) RegisterNode(id string, ip string, natType string) *Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	node := &Node{
		ID:       id,
		IP:       ip,
		NATType:  natType,
		LastSeen: time.Now(),
	}
	r.nodeIndex[id] = node
	return node
}

// Heartbeat updates the LastSeen timestamp for a node.
// Returns false if the node isn't registered.
func (r *RingRegistry) Heartbeat(nodeID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	node, exists := r.nodeIndex[nodeID]
	if !exists {
		return false
	}
	node.LastSeen = time.Now()
	return true
}

// EvictStaleNodes removes nodes that haven't sent a heartbeat within the timeout.
func (r *RingRegistry) EvictStaleNodes(timeout time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	evicted := 0
	for id, node := range r.nodeIndex {
		if time.Since(node.LastSeen) > timeout {
			delete(r.nodeIndex, id)
			evicted++
		}
	}
	return evicted
}
