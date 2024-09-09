package dgraph

type DependencyType string

const (
	// OrDependency means the dependencies are resolved when all `AND`s and a single `OR` dependency is resolved.
	// In other words, one OR short-circuits all ORs, but not ANDs.
	OrDependency DependencyType = "or"
	// AndDependency means the dependency is required for resolution.
	AndDependency DependencyType = "and"
	// CompletionAndDependency means the dependency will resolve due to either resolution or failure.
	CompletionAndDependency DependencyType = "completion-and"
	// ObviatedDependency is for dependencies that no longer have an effect due to a prior resolution.
	// For example, if one OR is resolved, all other OR dependencies are changed to ObviatedDependency.
	ObviatedDependency DependencyType = "obviated"
)

// ResolutionStatus indicates the individual status of the node.
// All nodes start out in Waiting ("waiting") status.
// The user of the DAG indicates when a node is resolved with `Node#ResolveNode()`,
// allowing dependencies to become marked as ready (which is separate from being resolved).
// As a convention, a status is only set once the node is ready, but that is not enforced.
type ResolutionStatus string

const (
	Waiting      ResolutionStatus = "waiting"
	Resolved     ResolutionStatus = "resolved"
	Unresolvable ResolutionStatus = "unresolvable"
)

// DirectedGraph is the representation of a Directed Graph width nodes and directed connections.
type DirectedGraph[NodeType any] interface {
	// AddNode adds a node with the specified ID. If the node already exists, it returns an ErrNodeAlreadyExists.
	AddNode(id string, item NodeType) (Node[NodeType], error)
	// GetNodeByID returns a node with the specified ID. If the specified node does not exist, an ErrNodeNotFound is
	// returned.
	GetNodeByID(id string) (Node[NodeType], error)
	// ListNodes lists all nodes in the graph.
	ListNodes() map[string]Node[NodeType]
	// ListNodesWithoutInboundConnections lists all nodes that do not have an inbound connection. This is useful for
	// performing a topological sort.
	ListNodesWithoutInboundConnections() map[string]Node[NodeType]
	// Clone creates an independent copy of the current directed graph.
	Clone() DirectedGraph[NodeType]
	// HasCycles performs cycle detection and returns true if the DirectedGraph has cycles.
	HasCycles() bool
	// PopReadyNodes returns of a list of all nodes that have no outstanding required dependencies,
	// and are therefore ready, and clears the list. Statuses may be stale after return.
	// A node becomes ready when all of its AND dependencies and at least one of
	// its OR dependencies are resolved.
	// Note that the resolution state of a node is independent of its readiness and that the
	// status varies depending on the behavior of the calling code.
	PopReadyNodes() map[string]ResolutionStatus
	// HasReadyNodes checks to see if there are any ready nodes without clearing them.
	HasReadyNodes() bool
	// PushStartingNodes initializes the list which is retrieved using `PopReadyNodes()`.
	// Recommended to be called only once following construction of the DAG.
	PushStartingNodes() error

	// Mermaid outputs the graph as a Mermaid string.
	Mermaid() string
}

// Node is a single point in a DirectedGraph.
type Node[NodeType any] interface {
	// ID returns the unique identifier of the node in the DG.
	ID() string
	// Item returns the underlying item for the node.
	Item() NodeType
	// Connect creates a new connection from the current node to the specified node.
	// If the specified node does not exist, ErrNodeNotFound is returned. If fromNodeID is equal to the node's ID,
	// ErrCannotConnectToSelf is returned.
	Connect(toNodeID string) error
	// ConnectDependency creates a new connection from the specified node to the current node.
	// The dependency type is set to determine when the node becomes finalized.
	// If the specified node does not exist, ErrNodeNotFound is returned. If fromNodeID is equal to the node's ID,
	// ErrCannotConnectToSelf is returned.
	ConnectDependency(fromNodeID string, dependencyType DependencyType) error
	// DisconnectInbound removes an incoming connection from the specified node. If the connection does not exist, an
	// ErrConnectionDoesNotExist is returned.
	DisconnectInbound(fromNodeID string) error
	// DisconnectOutbound removes an outgoing connection to the specified node. If the connection does not exist, an
	// ErrConnectionDoesNotExist is returned.
	DisconnectOutbound(toNodeID string) error
	// Remove removes the current node and all connections from the DirectedGraph.
	Remove() error
	// ListInboundConnections lists all inbound connections to this node.
	ListInboundConnections() (map[string]Node[NodeType], error)
	// ListOutboundConnections lists all outbound connections from this node.
	ListOutboundConnections() (map[string]Node[NodeType], error)
	// ResolveNode sets the resolution status of the node, and updates the nodes that follow it in the graph.
	// The resolution must happen only one time, or else a ErrNodeResolutionAlreadySet is returned.
	// This transitions the resolution status from the existing state (typically Waiting) to the given state.
	ResolveNode(status ResolutionStatus) error
	// OutstandingDependencies returns a map of the dependency node ID to the DependencyType of all dependencies
	// that have not been resolved yet.
	OutstandingDependencies() map[string]DependencyType
	// ResolvedDependencies returns a map of the dependency node ID to the DependencyType of all dependencies that
	// have been marked resolvable. The first OR resolved, if present, will retain its OR dependency type, but all
	// following OR resolutions will be marked as Obviated.
	ResolvedDependencies() map[string]DependencyType
}
