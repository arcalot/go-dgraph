package dgraph

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// New creates a new directed acyclic graph.
func New[NodeType any]() DirectedGraph[NodeType] {
	return &directedGraph[NodeType]{
		&sync.Mutex{},
		map[string]*node[NodeType]{},
		map[string]map[string]struct{}{},
		map[string]map[string]struct{}{},
	}
}

type directedGraph[NodeType any] struct {
	lock                *sync.Mutex
	nodes               map[string]*node[NodeType]
	connectionsFromNode map[string]map[string]struct{}
	connectionsToNode   map[string]map[string]struct{}
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
		d.cloneMap(d.connectionsFromNode),
		d.cloneMap(d.connectionsToNode),
	}

	for nodeID, nodeData := range d.nodes {
		newDG.nodes[nodeID] = &node[NodeType]{
			deleted: nodeData.deleted,
			id:      nodeID,
			item:    nodeData.item,
			dg:      newDG,
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
		false,
		id,
		item,
		false,
		WaitingForDependencies,
		make(map[string]DependencyType),
		d,
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

func (d *directedGraph[NodeType]) connectNodes(fromID, toID string, dependencyType DependencyType) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	// Make sure both nodes exist and are not deleted.
	fromNode, ok := d.nodes[fromID]
	if !ok {
		return &ErrNodeNotFound{
			fromID,
		}
	} else if fromNode.deleted {
		return &ErrNodeDeleted{
			fromID,
		}
	}
	toNode, ok := d.nodes[toID]
	if !ok {
		return &ErrNodeNotFound{
			toID,
		}
	} else if toNode.deleted {
		return &ErrNodeDeleted{
			toID,
		}
	}
	// Validate that it's a valid, non-duplicate, connection.
	if fromID == toID {
		return &ErrCannotConnectToSelf{
			fromID,
		}
	}
	if _, ok := d.connectionsFromNode[fromID][toID]; ok {
		return &ErrConnectionAlreadyExists{
			fromID,
			toID,
		}
	}
	// Update the mappings.
	d.connectionsFromNode[fromID][toID] = struct{}{}
	d.connectionsToNode[toID][fromID] = struct{}{}
	// Update the dependencies
	toNode.outstandingDependencies[fromID] = dependencyType
	return nil
}

type node[NodeType any] struct {
	deleted                 bool
	id                      string
	item                    NodeType
	finalized               bool
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

func (n *node[NodeType]) ResolveNode(status ResolutionStatus) error {
	if n.status != WaitingForDependencies {
		return ErrNodeResolutionAlreadySet{n.id, n.status, status}
	}
	n.status = status

	return nil
}

func (n *node[NodeType]) Connect(nodeID string) error {
	return n.dg.connectNodes(n.id, nodeID, AndDependency)
	/*n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return &ErrNodeDeleted{
			n.id,
		}
	}
	if nodeID == n.id {
		return &ErrCannotConnectToSelf{
			nodeID,
		}
	}
	if _, ok := n.dg.nodes[nodeID]; !ok {
		return &ErrNodeNotFound{
			nodeID,
		}
	}
	if _, ok := n.dg.connectionsFromNode[n.id][nodeID]; ok {
		return &ErrConnectionAlreadyExists{
			n.id,
			nodeID,
		}
	}
	n.dg.connectionsFromNode[n.id][nodeID] = struct{}{}
	n.dg.connectionsToNode[nodeID][n.id] = struct{}{}
	return nil*/
}

func (n *node[NodeType]) ConnectDependency(fromNodeID string, dependencyType DependencyType) error {
	return n.dg.connectNodes(fromNodeID, n.id, dependencyType)
}

func (n *node[NodeType]) DisconnectInbound(fromNodeID string) error {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return &ErrNodeDeleted{
			n.id,
		}
	}
	if _, ok := n.dg.nodes[fromNodeID]; !ok {
		return &ErrNodeNotFound{
			fromNodeID,
		}
	}
	if _, ok := n.dg.connectionsToNode[n.id][fromNodeID]; !ok {
		return &ErrConnectionDoesNotExist{
			n.id,
			fromNodeID,
		}
	}
	delete(n.dg.connectionsToNode[n.id], fromNodeID)
	delete(n.dg.connectionsFromNode[fromNodeID], n.id)
	return nil
}

func (n *node[NodeType]) DisconnectOutbound(toNodeID string) error {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return &ErrNodeDeleted{
			n.id,
		}
	}
	if _, ok := n.dg.nodes[toNodeID]; !ok {
		return &ErrNodeNotFound{
			toNodeID,
		}
	}
	if _, ok := n.dg.connectionsFromNode[n.id][toNodeID]; !ok {
		return &ErrConnectionDoesNotExist{
			n.id,
			toNodeID,
		}
	}
	delete(n.dg.connectionsFromNode[n.id], toNodeID)
	delete(n.dg.connectionsToNode[toNodeID], n.id)
	return nil
}

func (n *node[NodeType]) Remove() error {
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	if n.deleted {
		return &ErrNodeDeleted{
			n.id,
		}
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
		return nil, &ErrNodeDeleted{
			n.id,
		}
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
		return nil, &ErrNodeDeleted{
			n.id,
		}
	}
	result := make(map[string]Node[NodeType], len(n.dg.connectionsFromNode[n.id]))
	for toNodeID := range n.dg.connectionsFromNode[n.id] {
		result[toNodeID] = n.dg.nodes[toNodeID]
	}
	return result, nil
}

func (n *node[NodeType]) DependencyResolved(dependencyNodeID string, dependencyResolution ResolutionStatus) error {
	// TODO: Should we create a lock per node, and only sync the shared one when needed?
	n.dg.lock.Lock()
	defer n.dg.lock.Unlock()
	// TODO: Should we fail, or just stop propagation if deleted?
	if n.deleted {
		return &ErrNodeDeleted{
			n.id,
		}
	}
	if dependencyResolution == WaitingForDependencies {
		// Illegal state
		return ErrNotifiedOfWaiting{n.id, dependencyNodeID}
	}
	dependencyType, isOutstandingDependency := n.outstandingDependencies[dependencyNodeID]
	if !isOutstandingDependency {
		// Now determine if the missing item was because the dependency was already resolved, or
		// because there was never a connection.
		_, isConnected := n.dg.connectionsToNode[n.id][dependencyNodeID]
		if isConnected {
			return ErrDuplicateDependencyResolution{n.id, dependencyNodeID}
		} else {
			return ErrConnectionDoesNotExist{dependencyNodeID, n.id}
		}
	}
	delete(n.outstandingDependencies, dependencyNodeID)
	if dependencyResolution == Unresolvable && dependencyType == AndDependency {
		// Missing requirement. Fail.
		// TODO
	}
	// Now determine if it's ready to be finalized (no more deferred dependencies).
	/*hasAndDependency := false
	for _, dependencyType := range n.outstandingDependencies {
		if dependencyType == AndDependency {
			hasAndDependency = true
			break
		}
	}*/
	// If resolution is unresolvable, fail if current type is AND, or if there are no remaining OR dependencies.
	// Wait if there are remaining AND dependencies, completion dependencies, or if
	// no OR dependencies are resolved.
	// Success if there are no outstanding `AND` dependencies or `completion` dependencies, and
	// the current type is OR and success.
	// Otherwise, do nothing at this stage.
}

func (n *node[NodeType]) hasOutstandingAndDependency() bool {
	for _, dependencyType := range n.outstandingDependencies {
		if dependencyType == AndDependency {
			return true
		}
	}
	return false
}
