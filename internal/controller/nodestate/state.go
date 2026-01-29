package nodestate

import (
	"hash/fnv"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

// UpdatePhase represents the current phase of a node's update process.
type UpdatePhase string

const (
	// UpdatePhasePending indicates the node is waiting to start its update.
	UpdatePhasePending UpdatePhase = "Pending"
	// UpdatePhaseDraining indicates workloads are being evicted from the node.
	UpdatePhaseDraining UpdatePhase = "Draining"
	// UpdatePhaseUpdating indicates the configuration is being applied to the node.
	UpdatePhaseUpdating UpdatePhase = "Updating"
	// UpdatePhaseRebooting indicates the node is rebooting after configuration changes.
	UpdatePhaseRebooting UpdatePhase = "Rebooting"
	// UpdatePhasePaused indicates the update is paused by pool configuration.
	UpdatePhasePaused UpdatePhase = "Paused"
	// UpdatePhaseCompleted indicates the node has finished updating.
	UpdatePhaseCompleted UpdatePhase = "Completed"
)

// NodeState represents the evaluated state of a single node's update progress.
// This is the internal representation maintained by the central controller,
// loosely coupled from the NodeProgressInsightStatus CRD structure.
type NodeState struct {
	// Identity
	Name string    // Node name (immutable key)
	UID  types.UID // Node UID for validation

	// Pool Association
	PoolRef ouev1alpha1.ResourceRef // MachineConfigPool reference
	Scope   ouev1alpha1.ScopeType   // ControlPlane or WorkerPool

	// Version Information
	Version        string // OCP version (from MachineConfig annotation)
	DesiredVersion string // Target version from ClusterVersion
	CurrentConfig  string // Current MachineConfig name
	DesiredConfig  string // Desired MachineConfig name

	// Status Conditions
	Conditions []metav1.Condition // Updating, Available, Degraded

	// Update Progress
	Phase   UpdatePhase // Current update phase
	Message string      // Human-readable status message

	// Internal Tracking (not exposed in CRD)
	LastEvaluated    time.Time // When state was last computed
	EvaluationCount  int64     // Number of evaluations for this node
	SourceGeneration int64     // Node resourceVersion that triggered evaluation
	StateHash        uint64    // Hash of state for change detection
}

// ComputeHash computes a hash of the node state for efficient change detection.
// The hash includes all fields that affect the external representation but excludes
// internal tracking fields like LastEvaluated and EvaluationCount.
func (s *NodeState) ComputeHash() uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s.Name))
	_, _ = h.Write([]byte(s.PoolRef.Name))
	_, _ = h.Write([]byte(s.PoolRef.Group))
	_, _ = h.Write([]byte(s.PoolRef.Resource))
	_, _ = h.Write([]byte(s.Scope))
	_, _ = h.Write([]byte(s.Version))
	_, _ = h.Write([]byte(s.DesiredVersion))
	_, _ = h.Write([]byte(s.CurrentConfig))
	_, _ = h.Write([]byte(s.DesiredConfig))
	_, _ = h.Write([]byte(s.Phase))
	_, _ = h.Write([]byte(s.Message))
	for _, c := range s.Conditions {
		_, _ = h.Write([]byte(c.Type))
		_, _ = h.Write([]byte(c.Status))
		_, _ = h.Write([]byte(c.Reason))
		_, _ = h.Write([]byte(c.Message))
	}
	return h.Sum64()
}

// Store provides thread-safe storage and retrieval of NodeState instances.
type Store struct {
	states sync.Map // map[string]*NodeState (key: node name)
}

// Get returns the current evaluated state for a node.
// Returns (state, true) if found, (nil, false) if not tracked.
func (s *Store) Get(nodeName string) (*NodeState, bool) {
	v, ok := s.states.Load(nodeName)
	if !ok {
		return nil, false
	}
	return v.(*NodeState), true
}

// Set stores or updates the state for a node.
func (s *Store) Set(nodeName string, state *NodeState) {
	s.states.Store(nodeName, state)
}

// Delete removes the state for a node.
// Returns true if the node was present and removed.
func (s *Store) Delete(nodeName string) bool {
	_, loaded := s.states.LoadAndDelete(nodeName)
	return loaded
}

// Range iterates over all stored node states.
// The function fn is called for each state; if it returns false, iteration stops.
func (s *Store) Range(fn func(name string, state *NodeState) bool) {
	s.states.Range(func(key, value any) bool {
		return fn(key.(string), value.(*NodeState))
	})
}

// Count returns the number of nodes currently tracked.
func (s *Store) Count() int {
	count := 0
	s.states.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// GetAll returns all currently tracked node states.
// Used for bulk operations like MCP summary calculations.
func (s *Store) GetAll() []*NodeState {
	var states []*NodeState
	s.states.Range(func(_, value any) bool {
		states = append(states, value.(*NodeState))
		return true
	})
	return states
}

// GetByPool returns all nodes belonging to a specific MachineConfigPool.
// Used by MCP insight controller to calculate pool summaries.
func (s *Store) GetByPool(poolName string) []*NodeState {
	var states []*NodeState
	s.states.Range(func(_, value any) bool {
		state := value.(*NodeState)
		if state.PoolRef.Name == poolName {
			states = append(states, state)
		}
		return true
	})
	return states
}
