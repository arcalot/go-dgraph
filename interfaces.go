package dgraph

type DependencyType string

const (
	// OrDependency means the dependencies are resolved when all `AND`s and a single `OR` dependency is resolved.
	OrDependency DependencyType = "or"
	// AndDependency means the dependency is required no matter what.
	AndDependency DependencyType = "and"
	// CompletionDependency means the dependency will resolve due to either resolution or failure.
	CompletionDependency DependencyType = "completion"
	// SoftDependency means the dependency does not wait for this dependency to resolve.
	// The dependency may be unresolved at the time of resolution.
	SoftDependency DependencyType = "soft"
)

// ResolutionStatus indicates the status of the node for situations
//
// The way the resolution system works is first the DAG is created, with
// all nodes created, and all of their dependencies set.
type ResolutionStatus string

const (
	WaitingForDependencies ResolutionStatus = "waiting"
	Resolved               ResolutionStatus = "resolved"
	Unresolvable           ResolutionStatus = "unresolvable"
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
	// ListFinalizedNodes lists all nodes that have finalized their status, whether resolved or unresolvable.
	//ListFinalizedNodes()

	// Mermaid outputs the graph as a Mermaid string.
	Mermaid() string
}

// Node is a single point in a DirectedGraph.
type Node[NodeType any] interface {
	// ID returns the unique identifier of the node in the DG.
	ID() string
	// Item returns the underlying item for the node.
	Item() NodeType
	// Connect creates a new connection from the current node to the specified node. If the specified node does not
	// exist, ErrNodeNotFound is returned. If the connection had created a cycle, ErrConnectionWouldCreateACycle
	// is returned. Sets the dependency type to an AND dependency.
	Connect(toNodeID string) error
	// ConnectDependency creates a new connection from the specified node to the current node.
	// The dependency type is set to determine when the node becomes finalized.
	// If the specified node does not exist, ErrNodeNotFound is returned. If the connection had
	// created a cycle, ErrConnectionWouldCreateACycle is returned.
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
	// ResolveNode sets the resolution status of the node, and
	// updates the nodes that follow it in the graph.
	// The resolution must happen only one time, or else a ErrNodeResolutionAlreadySet is returned.
	ResolveNode(status ResolutionStatus) error
	// DependencyResolved is used to notify a node that one of its dependencies have had their resolution
	// status set. Once all dependencies are resolved, the node is set as finalized (ready for processing).
	DependencyResolved(dependencyNodeID string, dependencyResolution ResolutionStatus) error
}
