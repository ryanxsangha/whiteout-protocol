package main

import (
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

// AssignProxy is a stub — returns a hardcoded response for now.
// Real ring lookup logic will replace this later.
func (r *RingRegistry) AssignProxy(clientID string, natType string, premium bool) (*ProxyAssignment, error) {
	return &ProxyAssignment{
		RingID:       "ring-001",
		ProxyID:      "node-stub-001",
		ProxyIP:      "0.0.0.0",
		SpareID:      "node-stub-002",
		SpareIP:      "0.0.0.0",
		SessionToken: "stub-token-" + clientID,
	}, nil
}
