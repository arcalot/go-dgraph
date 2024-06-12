package dgraph_test

import (
	"testing"

	"go.arcalot.io/assert"
	"go.arcalot.io/dgraph"
)

func TestDirectedGraph_BasicNodeAdditionAndRemoval(t *testing.T) {
	d := dgraph.New[string]()
	n, err := d.AddNode("node-1", "Hello world!")
	assert.NoError(t, err)
	assert.Equals(t, n.ID(), "node-1")
	assert.Equals(t, n.Item(), "Hello world!")

	n2, err := d.GetNodeByID("node-1")
	assert.NoError(t, err)
	assert.Equals(t, n, n2)

	assert.ErrorR(t)(d.GetNodeByID("node-2"))

	nodes := d.ListNodesWithoutInboundConnections()
	assert.Equals(t, len(nodes), 1)

	assert.NoError(t, n.Remove())

	nodes = d.ListNodesWithoutInboundConnections()
	assert.Equals(t, len(nodes), 0)
	assert.ErrorR(t)(d.GetNodeByID("node-1"))
}

func TestDirectedGraph_ConnectSelf(t *testing.T) {
	d := dgraph.New[string]()
	n, err := d.AddNode("node-1", "Hello world!")
	assert.NoError(t, err)
	assert.Equals(t, n.ID(), "node-1")
	assert.Equals(t, n.Item(), "Hello world!")

	assert.Error(t, n.Connect("node-1"))
}

func TestDirectedGraph_Connect(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)

	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)

	t.Run("connect", func(t *testing.T) {
		assert.NoError(t, n1.Connect(n2.ID()))
		assert.Error(t, n1.Connect(n2.ID()))
		n1In, err := n1.ListInboundConnections()
		assert.NoError(t, err)
		assert.Equals(t, len(n1In), 0)
		n1Out, err := n1.ListOutboundConnections()
		assert.NoError(t, err)
		assert.Equals(t, len(n1Out), 1)
		assert.Equals(t, n1Out["node-2"].ID(), "node-2")
		n2In, err := n2.ListInboundConnections()
		assert.NoError(t, err)
		assert.Equals(t, len(n2In), 1)
		assert.Equals(t, n2In["node-1"].ID(), "node-1")
		n2Out, err := n2.ListOutboundConnections()
		assert.NoError(t, err)
		assert.Equals(t, len(n2Out), 0)
		starterNodes := d.ListNodesWithoutInboundConnections()
		assert.Equals(t, len(starterNodes), 1)
		assert.Equals(t, starterNodes["node-1"].ID(), "node-1")
	})
	t.Run("disconnect", func(t *testing.T) {
		assert.NoError(t, n2.DisconnectInbound(n1.ID()))
		n1In, err := n1.ListInboundConnections()
		assert.NoError(t, err)
		assert.Equals(t, len(n1In), 0)
		n1Out, err := n1.ListOutboundConnections()
		assert.NoError(t, err)
		assert.Equals(t, len(n1Out), 0)

		n2In, err := n2.ListInboundConnections()
		assert.NoError(t, err)
		assert.Equals(t, len(n2In), 0)
		n2Out, err := n2.ListOutboundConnections()
		assert.NoError(t, err)
		assert.Equals(t, len(n2Out), 0)

		starterNodes := d.ListNodesWithoutInboundConnections()
		assert.Equals(t, len(starterNodes), 2)
		assert.Equals(t, starterNodes["node-1"].ID(), "node-1")
		assert.Equals(t, starterNodes["node-2"].ID(), "node-2")
	})
}

func TestDirectedGraph_Clone(t *testing.T) {
	d := dgraph.New[string]()
	_, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)

	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)

	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	assert.NoError(t, n3.Connect(n2.ID()))

	d2 := d.Clone()

	d2n2, err := d2.GetNodeByID("node-2")
	assert.NoError(t, err)
	assert.NoError(t, d2n2.Remove())

	assert.Equals(t, len(d.ListNodesWithoutInboundConnections()), 2)
	assert.Equals(t, len(d2.ListNodesWithoutInboundConnections()), 2)
	n3Out, err := n3.ListOutboundConnections()
	assert.NoError(t, err)
	assert.Equals(t, len(n3Out), 1)
}

func TestDirectedGraph_HasCycles(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)
	assert.NoError(t, n1.Connect(n2.ID()))
	assert.NoError(t, n2.Connect(n3.ID()))
	assert.Equals(t, d.HasCycles(), false)
	assert.NoError(t, n3.Connect(n2.ID()))
	assert.Equals(t, d.HasCycles(), true)
	assert.NoError(t, n2.DisconnectOutbound(n3.ID()))
	assert.Equals(t, d.HasCycles(), false)
	assert.NoError(t, n2.Connect(n1.ID()))
	assert.Equals(t, d.HasCycles(), true)
}

// testSingleResolutionDependency() is a helper function for implementing test scenarios which
// consist of two nodes (n1 and n2) where n2 becomes ready when n1 resolves. The specified
// function parameter closure allows the caller to establish, check, and/or control the connection
// between the two nodes.
func testSingleResolutionDependency(
	t *testing.T,
	dependencyNodeResolution dgraph.ResolutionStatus,
	expectedDependentNodeResolution dgraph.ResolutionStatus,
	closure func(dependentNode dgraph.Node[string], dependencyNode dgraph.Node[string]),
) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "Dependent Node")
	assert.NoError(t, err)

	dependency1Node, err := d.AddNode("dependency-1", "Dependency 1")
	assert.NoError(t, err)

	closure(dependentNode, dependency1Node)

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependency1Node.ID(), readyNodes)
	assert.NoError(t, dependency1Node.ResolveNode(dependencyNodeResolution))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
}

func TestDirectedGraph_OneAndDependencyConnect(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Resolved, dgraph.Waiting,
		func(dependentNode dgraph.Node[string], dependencyNode dgraph.Node[string]) {
			// Use simple connect method, which will set an AND dependency.
			assert.NoError(t, dependencyNode.Connect(dependentNode.ID()))
		})
}

func TestDirectedGraph_OneAndDependencyConnectDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Resolved, dgraph.Waiting,
		func(dependentNode dgraph.Node[string], dependencyNode dgraph.Node[string]) {
			assert.NoError(t, dependentNode.ConnectDependency(dependencyNode.ID(), dgraph.AndDependency))
		})
}

func TestDirectedGraph_TwoAndDependencies(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "Dependent Node")
	assert.NoError(t, err)
	dependencyNode1, err := d.AddNode("dependency-node-1", "Dependency 1")
	assert.NoError(t, err)
	dependencyNode2, err := d.AddNode("dependency-node-2", "Dependency 2")
	assert.NoError(t, err)

	// dependentNode depends on dependencyNode1 and dependencyNode2.
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.AndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode2.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.MapContainsKey(t, dependencyNode1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode2.ID(), readyNodes)
	// Resolve the first. This isn't enough to fulfill the dependencies.
	assert.NoError(t, dependencyNode1.ResolveNode(dgraph.Resolved))
	// There should be no ready nodes. Test both ways of checking.
	assert.Equals(t, d.HasReadyNodes(), false)
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 0)
	// Resolve the second. This should now fulfill the dependencies.
	assert.NoError(t, dependencyNode2.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
}

func TestDirectedGraph_ChainedAndDependencies(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "Dependent Node")
	assert.NoError(t, err)
	// The middle node is both a dependency and a dependent.
	middleNode, err := d.AddNode("middle-node", "Middle Node")
	assert.NoError(t, err)
	dependencyNode, err := d.AddNode("dependency-node", "Dependency Node")
	assert.NoError(t, err)

	assert.NoError(t, dependentNode.ConnectDependency(middleNode.ID(), dgraph.AndDependency))
	assert.NoError(t, middleNode.ConnectDependency(dependencyNode.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependencyNode.ID(), readyNodes)
	// First, resolve the first dependency. This should make the middle node ready.
	assert.NoError(t, dependencyNode.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, middleNode.ID(), readyNodes)

	// Now that the middle node is ready, resolve it, and expect the dependent node to become ready.
	assert.NoError(t, middleNode.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)

}

func TestDirectedGraph_OneOrDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Resolved, dgraph.Waiting,
		func(dependentNode dgraph.Node[string], dependencyNode dgraph.Node[string]) {
			assert.NoError(t, dependentNode.ConnectDependency(dependencyNode.ID(), dgraph.OrDependency))
		},
	)
}

func TestDirectedGraph_TwoOrDependenciesResolveFirst(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "Dependent Node")
	assert.NoError(t, err)
	dependencyNode1, err := d.AddNode("dependency-node-1", "Dependency 1")
	assert.NoError(t, err)
	dependencyNode2, err := d.AddNode("dependency-node-2", "Dependency 2")
	assert.NoError(t, err)

	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode2.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.MapContainsKey(t, dependencyNode1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode2.ID(), readyNodes)
	// Resolve one node: dependencyNode1
	assert.NoError(t, dependencyNode1.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
}

func TestDirectedGraph_TwoOrDependenciesResolveSecond(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNode1, err := d.AddNode("dependency-node-1", "Dependency 1")
	assert.NoError(t, err)
	dependencyNode2, err := d.AddNode("dependency-node-2", "Dependency 2")
	assert.NoError(t, err)

	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode2.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.MapContainsKey(t, dependencyNode1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode2.ID(), readyNodes)
	// Resolve one node: dependencyNode2
	assert.NoError(t, dependencyNode2.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)

}

// Test two ANDs and two OR dependencies, with one OR resolving before the ANDs.
// Ensure resolution only marks dependentNode as ready at the correct resolution for the given dependency types.
func TestDirectedGraph_TwoOrAndTwoAndDependencies(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNodeOr1, err := d.AddNode("dependency-node-or-1", "Dependency OR 1")
	assert.NoError(t, err)
	dependencyNodeOr2, err := d.AddNode("dependency-node-or-2", "Dependency OR 2")
	assert.NoError(t, err)
	dependencyNodeAnd1, err := d.AddNode("dependency-node-and-1", "Dependency AND 1")
	assert.NoError(t, err)
	dependencyNodeAnd2, err := d.AddNode("dependency-node-and-2", "Dependency AND 2")
	assert.NoError(t, err)

	// (dependencyNodeOr1 || dependencyNodeOr2) && (dependencyNodeAnd1 && dependencyNodeAnd2)
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeOr1.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeOr2.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeAnd1.ID(), dgraph.AndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeAnd2.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 4)
	assert.MapContainsKey(t, dependencyNodeOr1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeOr2.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeAnd1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeAnd2.ID(), readyNodes)
	// Resolve one AND. There is another AND, so this should not make the dependent node ready.
	assert.NoError(t, dependencyNodeAnd1.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve one OR, dependencyNodeOr1. That alone is not enough for dependentNode to be ready because of
	// the remaining AND dependency.
	assert.NoError(t, dependencyNodeOr1.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve the final AND. This should result in the node being ready now.
	// We have now resolved one OR and both ANDs. One OR is enough, so there was no need
	// to resolve dependencyNodeOr2, too.
	assert.NoError(t, dependencyNodeAnd2.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)

}

// Test one AND and two OR dependencies, with both ORs resolving before the AND.
func TestDirectedGraph_BothOrAndOneAndDependencies(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNode1Or, err := d.AddNode("dependency-node-1-or", "Dependency 1 OR")
	assert.NoError(t, err)
	dependencyNode2Or, err := d.AddNode("dependency-node-2-or", "Dependency 2 OR")
	assert.NoError(t, err)
	dependencyNode3And, err := d.AddNode("dependency-node-3-and", "Dependency 3 AND")
	assert.NoError(t, err)

	// (dependencyNode1Or || dependencyNode2Or) && dependencyNode3And
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1Or.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode2Or.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode3And.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 3)
	assert.MapContainsKey(t, dependencyNode1Or.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode2Or.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode3And.ID(), readyNodes)
	// Resolve one OR. The dependentNode should not become ready because there is an unresolved AND.
	assert.NoError(t, dependencyNode1Or.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve the second OR. This should have no effect; still waiting on the AND.
	assert.NoError(t, dependencyNode2Or.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve the AND. This should make dependentNode ready.
	assert.NoError(t, dependencyNode3And.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)

}

// Test an AND and two OR dependencies, with the AND resolving before either OR.
func TestDirectedGraph_OneAndAndTwoOrDependencies(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNode1Or, err := d.AddNode("dependency-node-1-or", "Dependency 1 OR")
	assert.NoError(t, err)
	dependencyNode2Or, err := d.AddNode("dependency-node-2-or", "Dependency 2 OR")
	assert.NoError(t, err)
	dependencyNode3And, err := d.AddNode("dependency-node-3-and", "Dependency 3 AND")
	assert.NoError(t, err)

	// (dependencyNode1Or || dependencyNode2Or) && dependencyNode3And
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1Or.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode2Or.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode3And.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 3)
	assert.MapContainsKey(t, dependencyNode1Or.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode2Or.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode3And.ID(), readyNodes)

	// Resolve AND. It still needs the OR for dependentNode to become ready.
	assert.NoError(t, dependencyNode3And.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	// Resolve one OR. That should now be enough to make dependentNode ready.
	assert.NoError(t, dependencyNode1Or.ResolveNode(dgraph.Resolved))

	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)

}

// Test `Unresolvable` with a simple AND
func TestDirectedGraph_OneUnresolvableAndDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Unresolvable, dgraph.Unresolvable,
		func(dependentNode dgraph.Node[string], dependencyNode1 dgraph.Node[string]) {
			assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.AndDependency))
		},
	)
}

// Test `Unresolvable` with a simple OR.
func TestDirectedGraph_OneUnresolvableOrDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Unresolvable, dgraph.Unresolvable,
		func(dependentNode dgraph.Node[string], dependencyNode1 dgraph.Node[string]) {
			assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.OrDependency))
		},
	)
}

// Test two ANDs with one `Unresolvable`.
func TestDirectedGraph_TwoAndsOneUnresolvable(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNode1, err := d.AddNode("dependency-node-1", "Dependency 1")
	assert.NoError(t, err)
	dependencyNode2, err := d.AddNode("dependency-node-2", "Dependency 2")
	assert.NoError(t, err)

	// dependencyNode1 && dependencyNode2
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.AndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode2.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.MapContainsKey(t, dependencyNode1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode2.ID(), readyNodes)

	// Resolve one AND as `Unresolvable`. That should cause `dependentNode` to become ready and `Unresolvable`.
	assert.NoError(t, dependencyNode1.ResolveNode(dgraph.Unresolvable))

	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
	assert.Equals(t, readyNodes[dependentNode.ID()], dgraph.Unresolvable)
}

func TestDirectedGraph_ChainedAndDependenciesUnresolvable(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "Dependent Node")
	assert.NoError(t, err)
	// The middle node is both a dependency and a dependent.
	middleNode, err := d.AddNode("middle-node", "Middle Node")
	assert.NoError(t, err)
	dependencyNode, err := d.AddNode("dependency-node", "Dependency Node")
	assert.NoError(t, err)

	assert.NoError(t, dependentNode.ConnectDependency(middleNode.ID(), dgraph.AndDependency))
	assert.NoError(t, middleNode.ConnectDependency(dependencyNode.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependencyNode.ID(), readyNodes)
	// First, mark the first dependency as `Unresolvable`. This should make all nodes that
	// depend on it, directly or indirectly, `Unresolvable`, since none of the connections
	// have a completion dependency type.
	assert.NoError(t, dependencyNode.ResolveNode(dgraph.Unresolvable))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.MapContainsKey(t, middleNode.ID(), readyNodes)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
	assert.Equals(t, readyNodes[middleNode.ID()], dgraph.Unresolvable)
	assert.Equals(t, readyNodes[dependentNode.ID()], dgraph.Unresolvable)
}

// Test `Unresolvable` with two OR with one `Unresolvable` and one resolved.
func TestDirectedGraph_TwoOrsOneUnresolvable(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "Dependent Node")
	assert.NoError(t, err)
	dependencyNode1, err := d.AddNode("dependency-node-1", "Dependency 1")
	assert.NoError(t, err)
	dependencyNode2, err := d.AddNode("dependency-node-2", "Dependency 2")
	assert.NoError(t, err)

	// dependencyNode1 || dependencyNode2
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode2.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.MapContainsKey(t, dependencyNode1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNode2.ID(), readyNodes)

	// Resolve one OR as `Unresolvable`. That alone is not enough to cause dependentNode to
	// be marked `Unresolvable`.
	assert.NoError(t, dependencyNode1.ResolveNode(dgraph.Unresolvable))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	assert.NoError(t, dependencyNode2.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
	assert.Equals(t, readyNodes[dependentNode.ID()], dgraph.Waiting)
}

// Test `Unresolvable` with a simple completion dependency.
func TestDirectedGraph_OneUnresolvableCompletionDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Unresolvable, dgraph.Waiting,
		func(dependentNode dgraph.Node[string], dependencyNode1 dgraph.Node[string]) {
			assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.CompletionAndDependency))
		},
	)
}

// Test `Unresolvable` with a completion dependency and an AND.
func TestDirectedGraph_CompletionAndAnd(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNodeCompletion, err := d.AddNode("dependency-node-1", "Dependency 1")
	assert.NoError(t, err)
	dependencyNodeAnd, err := d.AddNode("dependency-node-2", "Dependency 2")
	assert.NoError(t, err)

	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeCompletion.ID(), dgraph.CompletionAndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeAnd.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.MapContainsKey(t, dependencyNodeCompletion.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeAnd.ID(), readyNodes)

	// Resolve the completion dependency as `Unresolvable`. Because the dependency type is
	// completion, dependentNode is not marked as `Unresolvable` and remains not ready
	// until the other AND dependency is resolved.
	assert.NoError(t, dependencyNodeCompletion.ResolveNode(dgraph.Unresolvable))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	assert.NoError(t, dependencyNodeAnd.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
	// Since it was not marked as unresolved, the status should not propagate. Only the readiness should propagate.
	assert.Equals(t, readyNodes[dependentNode.ID()], dgraph.Waiting)
}

// Test `Unresolvable` with a completion dependency and two OR, with the completion dependency `Unresolvable`.
func TestDirectedGraph_CompletionAndTwoOrs(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNodeCompletion, err := d.AddNode("dependency-node-1", "Dependency 1")
	assert.NoError(t, err)
	dependencyNodeOr1, err := d.AddNode("dependency-node-2", "Dependency 2")
	assert.NoError(t, err)
	dependencyNodeOr2, err := d.AddNode("dependency-node-3", "Dependency 3")
	assert.NoError(t, err)

	// dependencyNodeCompletion && (dependencyNodeOr1 || dependencyNodeOr2)
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeCompletion.ID(), dgraph.CompletionAndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeOr1.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeOr2.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 3)
	assert.MapContainsKey(t, dependencyNodeCompletion.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeOr1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeOr2.ID(), readyNodes)

	// Resolve the completion dependency as `Unresolvable`. Because the dependency type is
	// completion, dependentNode is not marked as `Unresolvable` and the node remains not
	// ready until an OR dependency is resolved.
	assert.NoError(t, dependencyNodeCompletion.ResolveNode(dgraph.Unresolvable))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	assert.NoError(t, dependencyNodeOr1.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
	// Since it was not marked as unresolved, the status should not propagate. Only the readiness should propagate.
	assert.Equals(t, readyNodes[dependentNode.ID()], dgraph.Waiting)
}

// Test `Unresolvable` with two ORs and two ANDs, with an AND being `Unresolvable`.
func TestDirectedGraph_UnresolvableAndsWithOrs(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNodeAnd1, err := d.AddNode("dependency-node-and-1", "Dependency AND 1")
	assert.NoError(t, err)
	dependencyNodeAnd2, err := d.AddNode("dependency-node-and-2", "Dependency AND 2")
	assert.NoError(t, err)
	dependencyNodeOr1, err := d.AddNode("dependency-node-or-1", "Dependency OR 1")
	assert.NoError(t, err)
	dependencyNodeOr2, err := d.AddNode("dependency-node-or-2", "Dependency OR 2")
	assert.NoError(t, err)

	// dependencyNodeAnd1 && dependencyNodeAnd2 && (dependencyNodeOr1 || dependencyNodeOr2)
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeAnd1.ID(), dgraph.AndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeAnd2.ID(), dgraph.AndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeOr1.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeOr2.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 4)
	assert.MapContainsKey(t, dependencyNodeAnd1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeAnd2.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeOr1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeOr2.ID(), readyNodes)

	// Resolve an AND as `Unresolvable`. This should cause instant propagation of the `Unresolvable`
	// state to dependentNode.
	assert.NoError(t, dependencyNodeAnd1.ResolveNode(dgraph.Unresolvable))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
	assert.Equals(t, readyNodes[dependentNode.ID()], dgraph.Unresolvable)
}

// Test `Unresolvable` with two ORs and two ANDs, with an OR being `Unresolvable`.
func TestDirectedGraph_UnresolvableOrsWithAnds(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNodeAnd1, err := d.AddNode("dependency-node-and-1", "Dependency AND 1")
	assert.NoError(t, err)
	dependencyNodeAnd2, err := d.AddNode("dependency-node-and-2", "Dependency AND 2")
	assert.NoError(t, err)
	dependencyNodeOr1, err := d.AddNode("dependency-node-or-1", "Dependency OR 1")
	assert.NoError(t, err)
	dependencyNodeOr2, err := d.AddNode("dependency-node-or-2", "Dependency OR 2")
	assert.NoError(t, err)

	// dependencyNodeAnd1 && dependencyNodeAnd2 && (dependencyNodeOr1 || dependencyNodeOr2)
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeAnd1.ID(), dgraph.AndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeAnd2.ID(), dgraph.AndDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeOr1.ID(), dgraph.OrDependency))
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNodeOr2.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 4)
	assert.MapContainsKey(t, dependencyNodeAnd1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeAnd2.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeOr1.ID(), readyNodes)
	assert.MapContainsKey(t, dependencyNodeOr2.ID(), readyNodes)

	assert.NoError(t, dependencyNodeOr1.ResolveNode(dgraph.Unresolvable))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	// Resolve the last OR as `Unresolvable`. This should cause instant propagation of the `Unresolvable`
	// state to dependentNode because none of the ORs could resolve, making dependentNode `Unresolvable`.
	assert.NoError(t, dependencyNodeOr2.ResolveNode(dgraph.Unresolvable))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.MapContainsKey(t, dependentNode.ID(), readyNodes)
	assert.Equals(t, readyNodes[dependentNode.ID()], dgraph.Unresolvable)
}

func TestDirectedGraph_TestResolvingDeletedNode(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	assert.NoError(t, n1.Remove())
	err = n1.ResolveNode(dgraph.Resolved)
	assert.Error(t, err)
	assert.InstanceOf[dgraph.ErrNodeDeleted](t, err)
}

func TestDirectedGraph_TestDoubleResolution(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	err = n1.ResolveNode(dgraph.Resolved)
	assert.NoError(t, err)
	err = n1.ResolveNode(dgraph.Resolved)
	assert.Error(t, err)
	assert.InstanceOf[dgraph.ErrNodeResolutionAlreadySet](t, err)
}

func TestDirectedGraph_TestWaitingResolution(t *testing.T) {
	d := dgraph.New[string]()
	dependentNode, err := d.AddNode("dependent-node", "dependent Node")
	assert.NoError(t, err)
	dependencyNode1, err := d.AddNode("dependency-node-1", "Dependency 1")
	assert.NoError(t, err)
	// Add a connection in case ResolveNode's behavior changes due to the presence of the connection.
	assert.NoError(t, dependentNode.ConnectDependency(dependencyNode1.ID(), dgraph.AndDependency))
	// Push and clear starting nodes.
	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	err = dependencyNode1.ResolveNode(dgraph.Waiting)
	assert.NoError(t, err)
	// It's waiting, still, so nothing should resolve.
	assert.Equals(t, d.HasReadyNodes(), false)
}

func TestDirectedGraph_PushStartingNodes(t *testing.T) {
	d := dgraph.New[string]()
	noDependencies, err := d.AddNode("no-dependencies", "No Dependencies")
	assert.NoError(t, err)
	onlyObviatedDependencies, err := d.AddNode("only-obviated-dependencies", "Only Obviated Dependencies")
	assert.NoError(t, err)
	withANDDependencies, err := d.AddNode("with-and-dependencies", "With AND Dependencies")
	assert.NoError(t, err)
	withORDependencies, err := d.AddNode("with-or-dependencies", "With OR Dependencies")
	assert.NoError(t, err)

	assert.NoError(t, withANDDependencies.ConnectDependency(noDependencies.ID(), dgraph.AndDependency))
	assert.NoError(t, withANDDependencies.ConnectDependency(onlyObviatedDependencies.ID(), dgraph.AndDependency))
	assert.NoError(t, withORDependencies.ConnectDependency(noDependencies.ID(), dgraph.OrDependency))
	assert.NoError(t, withORDependencies.ConnectDependency(onlyObviatedDependencies.ID(), dgraph.OrDependency))
	assert.NoError(t, onlyObviatedDependencies.ConnectDependency(noDependencies.ID(), dgraph.ObviatedDependency))
	assert.NoError(t, d.PushStartingNodes())
	assert.Equals(t, d.HasReadyNodes(), true)
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.MapContainsKey(t, noDependencies.ID(), readyNodes)
	// Obviated dependencies should not count, so it should have been marked ready.
	assert.MapContainsKey(t, onlyObviatedDependencies.ID(), readyNodes)
}
