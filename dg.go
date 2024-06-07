package dgraph

import (
	"fmt"
	"maps"
	"regexp"
	"strings"
	"sync"
)

// New creates a new directed acyclic graph.
func New[NodeType any]() DirectedGraph[NodeType] {
	return &directedGraph[NodeType]{
		&sync.Mutex{},
		map[string]*node[NodeType]{},
		map[string]*node[NodeType]{},
		map[string]map[string]struct{}{},
		map[string]map[string]struct{}{},
	}
}

type directedGraph[NodeType any] struct {
	lock               *sync.Mutex
	nodes              map[string]*node[NodeType]
	readyForProcessing map[string]*node[NodeType]
	// Map of the source nodes to a set of the destination nodes.
	connectionsFromNode map[string]map[string]struct{}
	// Map of the destination nodes to a set of the source nodes.
	connectionsToNode map[string]map[string]struct{}
}

var errorPathRegex, _ = regexp.Compile(`\.(?:error|crashed|failed|deploy_failed)$`)

func (d *directedGraph[NodeType]) Mermaid() string {
	result := []string{
		"%% Mermaid markdown workflow",
		"flowchart LR",
		"%% Success path",
	}

	errorPath := []string{"%% Error path"}

	for source, d := range d.connectionsFromNode {
		for destination := range d {
			isErrorPath := errorPathRegex.MatchString(destination)
			connection := fmt.Sprintf("%s-->%s", source, destination)
			if isErrorPath {
				errorPath = append(errorPath, connection)
			} else {
				result = append(result, connection)
			}
		}
	}

	result = append(result, errorPath...)

	result = append(result, "%% Mermaid end")
	return strings.Join(result, "\n") + "\n"
}

func (d *directedGraph[NodeType]) Clone() DirectedGraph[NodeType] {
	d.lock.Lock()
	defer d.lock.Unlock()

	newDG := &directedGraph[NodeType]{
		&sync.Mutex{},
		make(map[string]*node[NodeType], len(d.nodes)),
		make(map[string]*node[NodeType]), // Don't copy ready nodes.
		d.cloneMap(d.connectionsFromNode),
		d.cloneMap(d.connectionsToNode),
	}

	for nodeID, nodeData := range d.nodes {
		newDG.nodes[nodeID] = &node[NodeType]{
			deleted:                 nodeData.deleted,
			id:                      nodeID,
			item:                    nodeData.item,
			dg:                      newDG,
			ready:                   nodeData.ready,
			status:                  nodeData.status,
			outstandingDependencies: maps.Clone(nodeData.outstandingDependencies),
		}
	}

	return newDG
}

func (d *directedGraph[NodeType]) cloneMap(source map[string]map[string]struct{}) map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{}, len(source))
	for nodeID1, tier2 := range source {
		result[nodeID1] = make(map[string]struct{}, len(tier2))
		for nodeID2 := range tier2 {
			result[nodeID1][nodeID2] = struct{}{}
		}
	}
	return result
}

func (d *directedGraph[NodeType]) HasCycles() bool {
	connectionsToNode := d.cloneMap(d.connectionsToNode)
	for {
		var removeNodeIDs []string
		// Select all nodes that have no inbound connections
		for nodeID, inboundConnections := range connectionsToNode {
			if len(inboundConnections) == 0 {
				removeNodeIDs = append(removeNodeIDs, nodeID)
			}
		}
		// If no nodes without inbound connections are found...
		if len(removeNodeIDs) == 0 {
			// ...there is a cycle if there are nodes left
			return len(connectionsToNode) != 0
		}
		for _, nodeID := range removeNodeIDs {
			// Remove all previously-selected nodes
			delete(connectionsToNode, nodeID)
			// Remove connections from the selected nodes from the remaining nodes
		}
		for _, nodeID := range removeNodeIDs {
			for targetNodeID := range connectionsToNode {
				delete(connectionsToNode[targetNodeID], nodeID)
			}
		}
	}
}

func (d *directedGraph[NodeType]) AddNode(id string, item NodeType) (Node[NodeType], error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if _, ok := d.nodes[id]; ok {
		return nil, ErrNodeAlreadyExists{
			id,
		}
	}
	d.nodes[id] = &node[NodeType]{
		deleted:                 false,
		ready:                   false,
		id:                      id,
		item:                    item,
		status:                  Waiting,
		outstandingDependencies: make(map[string]DependencyType),
		dg:                      d,
	}
	d.connectionsToNode[id] = map[string]struct{}{}
	d.connectionsFromNode[id] = map[string]struct{}{}
	return d.nodes[id], nil
}

func (d *directedGraph[NodeType]) GetNodeByID(id string) (Node[NodeType], error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	n, ok := d.nodes[id]
	if !ok {
		return nil, &ErrNodeNotFound{
			id,
		}
	}
	return n, nil
}

func (d *directedGraph[NodeType]) ListNodes() map[string]Node[NodeType] {
	d.lock.Lock()
	defer d.lock.Unlock()

	result := map[string]Node[NodeType]{}
	for nodeID, n := range d.nodes {
		result[nodeID] = n
	}
	return result
}

func (d *directedGraph[NodeType]) ListNodesWithoutInboundConnections() map[string]Node[NodeType] {
	d.lock.Lock()
	defer d.lock.Unlock()

	result := map[string]Node[NodeType]{}
	for nodeID, n := range d.nodes {
		connections := d.connectionsToNode[nodeID]
		if len(connections) == 0 {
			result[nodeID] = n
		}
	}
	return result
}

// Validates the specified node IDs and confirms that a connection between them
// would be valid, then sets the `to` and `from` connections and adds the
// dependency to the `to` node.
func (d *directedGraph[NodeType]) connectNodes(fromID, toID string, dependencyType DependencyType) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	// Make sure both nodes exist and are not deleted.
	fromNode, ok := d.nodes[fromID]
	if !ok {
		return &ErrNodeNotFound{fromID}
	} else if fromNode.deleted {
		return &ErrNodeDeleted{fromID}
	}
	toNode, ok := d.nodes[toID]
	if !ok {
		return &ErrNodeNotFound{toID}
	} else if toNode.deleted {
		return &ErrNodeDeleted{toID}
	}
	// Check that it's a non-self and non-duplicate connection.
	if fromID == toID {
		return &ErrCannotConnectToSelf{fromID}
	}
	if _, ok := d.connectionsFromNode[fromID][toID]; ok {
		return &ErrConnectionAlreadyExists{fromID, toID}
	}
	// Update the mappings.
	d.connectionsFromNode[fromID][toID] = struct{}{}
	d.connectionsToNode[toID][fromID] = struct{}{}
	// Update the dependencies
	toNode.outstandingDependencies[fromID] = dependencyType
	return nil
}

func (d *directedGraph[NodeType]) PushStartingNodes() error {
	d.lock.Lock()
	defer d.lock.Unlock()

	for nodeID, n := range d.nodes {
		if len(n.outstandingDependencies) == 0 {
			d.readyForProcessing[nodeID] = n
		}
	}
	return nil
}

func (d *directedGraph[NodeType]) PopReadyNodes() []*node[NodeType] {
	d.lock.Lock()
	// Transfer the map to a local variable to minimize time locked, and reset the graph's value.
	readyMap := d.readyForProcessing
	d.readyForProcessing = make(map[string]*node[NodeType])
	d.lock.Unlock()

	result := make([]*node[NodeType], len(readyMap))
	i := 0
	for _, node := range readyMap {
		result[i] = node
		i += 1
	}
	return result
}

type node[NodeType any] struct {
	deleted                 bool
	id                      string
	item                    NodeType
	ready                   bool
	status                  ResolutionStatus
	outstandingDependencies map[string]DependencyType
	dg                      *directedGraph[NodeType]
}

func (n *node[NodeType]) ID() string {
	return n.id
}

func (n *node[NodeType]) Item() NodeType {
	return n.item
}

func (n *node[NodeType]) ResolutionStatus() ResolutionStatus {
	// The status can change and be accessed on different threads, so lock to prevent races.
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	return n.status
}

func (n *node[NodeType]) IsReady() bool {
	// The status can change and be accessed on different threads, so lock to prevent races.
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	return n.ready
}

func (n *node[NodeType]) OutstandingDependencies() map[string]DependencyType {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	return maps.Clone(n.outstandingDependencies)
}

// ResolveNode is the externally accessible way to resolve the node.
// This function will take care of the locking, then call the internal
// resolveNode function.
func (n *node[NodeType]) ResolveNode(status ResolutionStatus) error {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	return n.resolveNode(status)
}

// Caller should have appropriate mutex locked before calling.
func (n *node[NodeType]) resolveNode(status ResolutionStatus) error {
	if n.deleted {
		return ErrNodeDeleted{n.id}
	}
	if n.status == "" {
		return ErrNodeResolutionUnknown{n.id, n.status}
	}
	if n.status != Waiting {
		return ErrNodeResolutionAlreadySet{n.id, n.status, status}
	}
	n.status = status
	if status == Waiting {
		return nil // Don't propagate a waiting status.
	}
	// Propagate to outbound connections.
	outboundConnections := n.dg.connectionsFromNode[n.ID()]
	for outboundConnectionID := range outboundConnections {
		err := n.dg.nodes[outboundConnectionID].dependencyResolved(n.ID(), status)
		if err != nil {
			return err
		}
	}
	return nil
}

// Connect connects forward from the called node to the node with the ID specified
// in fromNodeID. It has an AndDependency type for legacy reasons.
func (n *node[NodeType]) Connect(nodeID string) error {
	return n.dg.connectNodes(n.id, nodeID, AndDependency)
}

// ConnectDependency connects backward and sets a dependency. The connection is made
// from the node with the ID specified to the called node.
func (n *node[NodeType]) ConnectDependency(fromNodeID string, dependencyType DependencyType) error {
	return n.dg.connectNodes(fromNodeID, n.id, dependencyType)
}

func (n *node[NodeType]) DisconnectInbound(fromNodeID string) error {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return &ErrNodeDeleted{n.id}
	}
	if _, ok := n.dg.nodes[fromNodeID]; !ok {
		return &ErrNodeNotFound{fromNodeID}
	}
	if _, ok := n.dg.connectionsToNode[n.id][fromNodeID]; !ok {
		return &ErrConnectionDoesNotExist{n.id, fromNodeID}
	}
	delete(n.dg.connectionsToNode[n.id], fromNodeID)
	delete(n.dg.connectionsFromNode[fromNodeID], n.id)
	return nil
}

func (n *node[NodeType]) DisconnectOutbound(toNodeID string) error {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return &ErrNodeDeleted{n.id}
	}
	if _, ok := n.dg.nodes[toNodeID]; !ok {
		return &ErrNodeNotFound{toNodeID}
	}
	if _, ok := n.dg.connectionsFromNode[n.id][toNodeID]; !ok {
		return &ErrConnectionDoesNotExist{n.id, toNodeID}
	}
	delete(n.dg.connectionsFromNode[n.id], toNodeID)
	delete(n.dg.connectionsToNode[toNodeID], n.id)
	return nil
}

func (n *node[NodeType]) Remove() error {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return &ErrNodeDeleted{n.id}
	}
	for toNodeID := range n.dg.connectionsFromNode[n.id] {
		delete(n.dg.connectionsToNode[toNodeID], n.id)
	}
	delete(n.dg.connectionsFromNode, n.id)
	for fromNodeID := range n.dg.connectionsToNode[n.id] {
		delete(n.dg.connectionsFromNode[fromNodeID], n.id)
	}
	delete(n.dg.connectionsToNode, n.id)
	delete(n.dg.nodes, n.id)
	n.deleted = true
	return nil
}

func (n *node[NodeType]) ListInboundConnections() (map[string]Node[NodeType], error) {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return nil, &ErrNodeDeleted{n.id}
	}
	result := make(map[string]Node[NodeType], len(n.dg.connectionsToNode[n.id]))
	for fromNodeID := range n.dg.connectionsToNode[n.id] {
		result[fromNodeID] = n.dg.nodes[fromNodeID]
	}
	return result, nil
}

func (n *node[NodeType]) ListOutboundConnections() (map[string]Node[NodeType], error) {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return nil, &ErrNodeDeleted{n.id}
	}
	result := make(map[string]Node[NodeType], len(n.dg.connectionsFromNode[n.id]))
	for toNodeID := range n.dg.connectionsFromNode[n.id] {
		result[toNodeID] = n.dg.nodes[toNodeID]
	}
	return result, nil
}

// dependencyResolved is used to notify a node that one of that node's dependencies have had their resolution
// status set. Once all dependencies are resolved, the node is set as finalized (ready for processing).
// Caller should have appropriate mutex locked before calling.
func (n *node[NodeType]) dependencyResolved(dependencyNodeID string, dependencyResolution ResolutionStatus) error {
	if n.deleted {
		return &ErrNodeDeleted{n.id}
	}
	if dependencyResolution == Waiting {
		// Illegal state
		return ErrNotifiedOfWaiting{n.id, dependencyNodeID}
	}
	dependencyType, isOutstandingDependency := n.outstandingDependencies[dependencyNodeID]
	if !isOutstandingDependency {
		// Now determine if the missing item was because the dependency was already resolved, or
		// because there was never a connection.
		_, isConnected := n.dg.connectionsToNode[n.id][dependencyNodeID]
		if isConnected {
			// As designed, this is an internal function. So we guard against this in resolveNode.
			panic(ErrDuplicateDependencyResolution{n.id, dependencyNodeID})
		} else {
			panic(ErrConnectionDoesNotExist{dependencyNodeID, n.id})
		}
	}
	delete(n.outstandingDependencies, dependencyNodeID)
	if dependencyType == ObviatedDependency {
		return nil // Nothing to do.
	}
	// If the dependency is unresolvable, mark self as unresolvable if current type is AND,
	// or if there are no remaining OR dependencies.
	// Treat as resolved if is just a completion-AND dependency.
	if dependencyResolution == Unresolvable && dependencyType != CompletionAndDependency {
		// Check for the unresolvable case.
		if dependencyType == AndDependency || !n.hasOutstandingDependency(OrDependency) {
			// Missing requirement. Mark as unresolvable, which propagates to outbound connections.
			n.ready = true
			n.dg.readyForProcessing[n.id] = n
			return n.resolveNode(Unresolvable)
		}
	} else {
		var hasOrDependency bool
		if dependencyType == OrDependency {
			n.markOrsObviated()
			hasOrDependency = false // This resolved all outstanding ORs.
		} else {
			hasOrDependency = n.hasOutstandingDependency(OrDependency)
		}
		hasAndDependency := n.hasOutstandingDependency(AndDependency) || n.hasOutstandingDependency(CompletionAndDependency)
		// Now determine if it's ready to be finalized (no more deferred dependencies).
		if !(hasAndDependency || hasOrDependency) {
			// Mark as ready for processing internally and in the DAG.
			n.ready = true
			n.dg.readyForProcessing[n.id] = n
		}
	}
	return nil
}

// Caller should have appropriate mutex locked before calling.
func (n *node[NodeType]) markOrsObviated() {
	for dependency, dependencyType := range n.outstandingDependencies {
		if dependencyType == OrDependency {
			n.outstandingDependencies[dependency] = ObviatedDependency
		}
	}
}

// Caller should have appropriate mutex locked before calling.
func (n *node[NodeType]) hasOutstandingDependency(expectedDependencyType DependencyType) bool {
	for _, dependencyType := range n.outstandingDependencies {
		if dependencyType == expectedDependencyType {
			return true
		}
	}
	return false
}
