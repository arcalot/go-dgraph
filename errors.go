package dgraph

import "fmt"

// ErrNodeDeleted indicates that the current node has already been removed from the DirectedGraph.
type ErrNodeDeleted struct {
	NodeID string
}

func (e ErrNodeDeleted) Error() string {
	return fmt.Sprintf("node with ID %q is deleted", e.NodeID)
}

// ErrCannotConnectToSelf indicates that an attempt was made to connect a node to itself.
type ErrCannotConnectToSelf struct {
	NodeID string
}

func (e ErrCannotConnectToSelf) Error() string {
	return fmt.Sprintf("cannot connect node %q to itself", e.NodeID)
}

// ErrNodeNotFound is an error that is returned if the specified node is not found.
type ErrNodeNotFound struct {
	NodeID string
}

func (e ErrNodeNotFound) Error() string {
	return fmt.Sprintf("node with ID %q not found", e.NodeID)
}

// ErrNodeAlreadyExists signals that a node with the specified ID already exists.
type ErrNodeAlreadyExists struct {
	NodeID string
}

func (e ErrNodeAlreadyExists) Error() string {
	return fmt.Sprintf("node with ID %q already exists", e.NodeID)
}

// ErrConnectionWouldCreateACycle is an error that is returned if the newly created connection would create a cycle.
type ErrConnectionWouldCreateACycle struct {
	SourceNodeID      string
	DestinationNodeID string
}

func (e ErrConnectionWouldCreateACycle) Error() string {
	return fmt.Sprintf(
		"connection from node %q to node %q would create a cycle",
		e.SourceNodeID,
		e.DestinationNodeID,
	)
}

// ErrConnectionAlreadyExists indicates that the connection you are trying to create already exists.
type ErrConnectionAlreadyExists struct {
	SourceNodeID      string
	DestinationNodeID string
}

func (e ErrConnectionAlreadyExists) Error() string {
	return fmt.Sprintf(
		"connection from node %q to node %q already exists",
		e.SourceNodeID,
		e.DestinationNodeID,
	)
}

// ErrConnectionDoesNotExist is returned if the specified connection between the two nodes does not exist.
type ErrConnectionDoesNotExist struct {
	SourceNodeID      string
	DestinationNodeID string
}

func (e ErrConnectionDoesNotExist) Error() string {
	return fmt.Sprintf(
		"connection from node %q to node %q does not exist",
		e.SourceNodeID,
		e.DestinationNodeID,
	)
}

type ErrNodeResolutionAlreadySet struct {
	NodeID         string
	ExistingStatus ResolutionStatus
	NewStatus      ResolutionStatus
}

func (e ErrNodeResolutionAlreadySet) Error() string {
	return fmt.Sprintf(
		"attempted to re-resolve node %q with resolution %q; already set to %q",
		e.NodeID, e.NewStatus, e.ExistingStatus,
	)
}

type ErrNodeResolutionUnknown struct {
	NodeID         string
	ExistingStatus ResolutionStatus
}

func (e ErrNodeResolutionUnknown) Error() string {
	return fmt.Sprintf("while resolving node %q; the existing resolution field had an invalid value of %q",
		e.NodeID, e.ExistingStatus,
	)
}

type ErrDuplicateDependencyResolution struct {
	NodeID       string
	DependencyID string
}

func (e ErrDuplicateDependencyResolution) Error() string {
	return fmt.Sprintf(
		"attempted to re-resolve dependency %q of node %q; the connection remains; but there are"+
			" no outstanding requirements",
		e.NodeID, e.DependencyID,
	)
}

type ErrNotifiedOfWaiting struct {
	NodeID       string
	DependencyID string
}

func (e ErrNotifiedOfWaiting) Error() string {
	return fmt.Sprintf(
		"notified node %q of waiting resolution of dependency %q; expected a non-waiting state",
		e.NodeID, e.DependencyID,
	)
}
