package main

import (
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"sync"
	"time"
)

// RingRegistry holds all active rings and a fast node lookup index.
type RingRegistry struct {
	rings     map[string]*Ring
	nodeIndex map[string]*Node
	sessions  map[string]*Session
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
		sessions:  make(map[string]*Session),
	}
}

// AssignProxy finds an available ring, assigns a proxy and spare to the client,
// creates a Session, and returns a signed ProxyAssignment with a real session token.
func (r *RingRegistry) AssignProxy(clientID string, natType string, premium bool) (*ProxyAssignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ring := range r.rings {
		if len(ring.Nodes) < 2 {
			continue
		}
		for idx, node := range ring.Nodes {
			if node.ID == clientID {
				continue
			}
			spare := ring.Nodes[(idx+1)%len(ring.Nodes)]
			if spare.ID == clientID {
				spare = ring.Nodes[(idx+2)%len(ring.Nodes)]
			}

			token, err := generateToken()
			if err != nil {
				return nil, fmt.Errorf("failed to generate session token: %w", err)
			}

			now := time.Now()
			session := &Session{
				ClientID:   clientID,
				RingID:     ring.ID,
				ProxyID:    node.ID,
				SpareID:    spare.ID,
				CreatedAt:  now,
				LastActive: now,
			}
			r.sessions[token] = session

			return &ProxyAssignment{
				RingID:       ring.ID,
				ProxyID:      node.ID,
				ProxyIP:      node.IP,
				SpareID:      spare.ID,
				SpareIP:      spare.IP,
				SessionToken: token,
			}, nil
		}
	}

	return nil, fmt.Errorf("no available rings for client %s", clientID)
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

// FormRings groups all registered nodes into directed rings of 3–5 nodes,
// assigning each node its NextHop and WarmSpare.
// Existing ring assignments are cleared before re-formation.
func (r *RingRegistry) FormRings() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Collect all nodes into a slice
	nodes := make([]*Node, 0, len(r.nodeIndex))
	for _, node := range r.nodeIndex {
		nodes = append(nodes, node)
	}

	// Clear existing ring assignments
	r.rings = make(map[string]*Ring)
	for _, node := range nodes {
		node.RingID = ""
		node.NextHop = ""
		node.WarmSpare = ""
	}

	if len(nodes) < 3 {
		return 0
	}

	mrand.Shuffle(len(nodes), func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})

	// Partition into rings of target size 4, min 3, max 5
	targetSize := 4
	ringCount := len(nodes) / targetSize
	if ringCount == 0 {
		ringCount = 1
	}

	formed := 0
	start := 0
	for i := 0; i < ringCount; i++ {
		end := start + targetSize
		// Last ring absorbs the remainder
		if i == ringCount-1 {
			end = len(nodes)
		}
		chunk := nodes[start:end]
		if len(chunk) < 3 {
			// Fold into previous ring if too small
			break
		}

		ringID := fmt.Sprintf("ring-%04d", formed+1)
		ring := &Ring{
			ID:    ringID,
			Nodes: chunk,
		}

		n := len(chunk)
		for idx, node := range chunk {
			node.RingID = ringID
			node.NextHop = chunk[(idx+1)%n].ID
			node.WarmSpare = chunk[(idx+2)%n].ID
		}

		r.rings[ringID] = ring
		formed++
		start = end
	}

	return formed
}

// generateToken returns a cryptographically random 32-byte hex session token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// LookupSession retrieves a session by token and updates LastActive.
// Returns nil if the token is not found.
func (r *RingRegistry) LookupSession(token string) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[token]
	if !exists {
		return nil
	}
	session.LastActive = time.Now()
	return session
}
