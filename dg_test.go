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

// For tests when you have two nodes (n1 and n2), and n2 becomes ready when n1 resolves.
// Set the dependency connection with the closure.
func testSingleResolutionDependency(
	t *testing.T,
	N1Resolution dgraph.ResolutionStatus,
	expectedN2Resolution dgraph.ResolutionStatus,
	closure func(n1 dgraph.Node[string], n2 dgraph.Node[string]),
) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)

	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)

	closure(n1, n2)

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.NoError(t, n1.ResolveNode(N1Resolution))
	assert.Equals(t, n1.ResolutionStatus(), N1Resolution)
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n2.ID())
	assert.Equals(t, n2.ResolutionStatus(), expectedN2Resolution)
}

func TestDirectedGraph_OneAndDependencyConnect(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Resolved, dgraph.Waiting,
		func(n1 dgraph.Node[string], n2 dgraph.Node[string]) {
			// Use simple connect method, which will set an AND dependency.
			assert.NoError(t, n1.Connect(n2.ID()))
		})
}

func TestDirectedGraph_OneAndDependencyConnectDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Resolved, dgraph.Waiting,
		func(n1 dgraph.Node[string], n2 dgraph.Node[string]) {
			// Use the dependency connection method instead.
			assert.NoError(t, n2.ConnectDependency(n1.ID(), dgraph.AndDependency))
		})
}

// Test two AND dependencies.
func TestDirectedGraph_TwoAndDependencies(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	// Use the dependency connection method instead.
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.AndDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	// Resolve the first. This isn't enough to fulfill the dependencies.
	assert.NoError(t, n2.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 0)
	// Resolve the second. This should now fulfill the dependencies.
	assert.NoError(t, n3.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
}

// Test one OR dependency.
func TestDirectedGraph_OneOrDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Resolved, dgraph.Waiting,
		func(n1 dgraph.Node[string], n2 dgraph.Node[string]) {
			assert.NoError(t, n2.ConnectDependency(n1.ID(), dgraph.OrDependency))
		},
	)
}

// Test two OR dependencies.
func TestDirectedGraph_TwoOrDependenciesResolveFirst(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	// Resolve one node: n2
	assert.NoError(t, n2.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
}

func TestDirectedGraph_TwoOrDependenciesResolveSecond(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	// Resolve one node: n3
	assert.NoError(t, n3.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
}

// Test two ANDs and two OR dependencies, with one OR resolving before the ANDs.
func TestDirectedGraph_TwoOrAndTwoAndDependencies(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)
	n4, err := d.AddNode("node-4", "test4")
	assert.NoError(t, err)
	n5, err := d.AddNode("node-5", "test5")
	assert.NoError(t, err)

	// (N2 || N3) && (N4 && N5)
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n4.ID(), dgraph.AndDependency))
	assert.NoError(t, n1.ConnectDependency(n5.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 4)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	assert.SliceContains(t, n4, readyNodes)
	assert.SliceContains(t, n5, readyNodes)
	// Resolve one AND. There is another AND, so this should not resolve anything.
	assert.NoError(t, n4.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve one OR. That alone is not enough for n1 to be ready. No need to resolve n2, too.
	assert.NoError(t, n2.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve the final AND. This should result in the node being ready now.
	assert.NoError(t, n5.ResolveNode(dgraph.Resolved))

	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
}

// Test one AND and two OR dependencies, with both ORs resolving before the AND.
func TestDirectedGraph_BothOrAndOneAndDependencies(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)
	n4, err := d.AddNode("node-4", "test4")
	assert.NoError(t, err)

	// (N2 || N3) && N4
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n4.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 3)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	assert.SliceContains(t, n4, readyNodes)
	// Resolve one OR. Nothing should resolve because there is an unresolved AND.
	assert.NoError(t, n2.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve the second OR. This should have no effect; still waiting on the AND.
	assert.NoError(t, n3.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve the AND. This should resolve to make n1 ready.
	assert.NoError(t, n4.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
}

// Test an AND and two OR dependencies, with the AND resolving before one OR.
func TestDirectedGraph_OneAndAndTwoOrDependencies(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)
	n4, err := d.AddNode("node-4", "test4")
	assert.NoError(t, err)

	// (N2 || N3) && N4
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n4.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 3)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	assert.SliceContains(t, n4, readyNodes)

	// Resolve AND. It still needs the OR to resolve.
	assert.NoError(t, n4.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	// Resolve one OR. That should now be enough to resolve it.
	assert.NoError(t, n2.ResolveNode(dgraph.Resolved))

	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
}

// Test unresolvable with a simple AND
func TestDirectedGraph_OneUnresolvableAndDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Unresolvable, dgraph.Unresolvable,
		func(n1 dgraph.Node[string], n2 dgraph.Node[string]) {
			assert.NoError(t, n2.ConnectDependency(n1.ID(), dgraph.AndDependency))
		},
	)
}

// Test unresolvable with a simple OR.
func TestDirectedGraph_OneUnresolvableOrDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Unresolvable, dgraph.Unresolvable,
		func(n1 dgraph.Node[string], n2 dgraph.Node[string]) {
			assert.NoError(t, n2.ConnectDependency(n1.ID(), dgraph.OrDependency))
		},
	)
}

// Test unresolvable with two AND with one failure.
func TestDirectedGraph_TwoAndsOneFail(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	// N2 && N3
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.AndDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)

	// Resolve one AND as unresolvable. That should exit early as unresolvable.
	assert.NoError(t, n2.ResolveNode(dgraph.Unresolvable))

	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.Equals(t, readyNodes[0].ResolutionStatus(), dgraph.Unresolvable)
}

// Test unresolvable with two OR with one failure and one success.
func TestDirectedGraph_TwoOrsOneFail(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	// N2 || N3
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)

	// Resolve one OR as unresolvable. That alone is not enough to cause a problem.
	assert.NoError(t, n2.ResolveNode(dgraph.Unresolvable))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	assert.NoError(t, n3.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.Equals(t, readyNodes[0].ResolutionStatus(), dgraph.Waiting)
}

// Test unresolvable with a simple completion dependency.
func TestDirectedGraph_OneUnresolvableCompletionDependency(t *testing.T) {
	testSingleResolutionDependency(t, dgraph.Unresolvable, dgraph.Waiting,
		func(n1 dgraph.Node[string], n2 dgraph.Node[string]) {
			assert.NoError(t, n2.ConnectDependency(n1.ID(), dgraph.CompletionDependency))
		},
	)
}

// Test unresolvable with a completion dependency and an AND.
func TestDirectedGraph_CompletionAndAnd(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.CompletionDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 2)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)

	// Resolve the completion dependency as unresolvable. That is equivalent to setting an AND dependency as resolved.
	assert.NoError(t, n2.ResolveNode(dgraph.Unresolvable))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	assert.NoError(t, n3.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.Equals(t, readyNodes[0].ResolutionStatus(), dgraph.Waiting)
}

// Test unresolvable with a completion dependency and two OR, with the completion dependency unresolvable.
func TestDirectedGraph_CompletionAndTwoOrs(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)
	n4, err := d.AddNode("node-4", "test4")
	assert.NoError(t, err)

	// N2 && (N3 || N4)
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.CompletionDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n4.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 3)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	assert.SliceContains(t, n4, readyNodes)

	// Resolve the completion dependency as unresolvable. That is equivalent to setting an AND dependency as resolved.
	assert.NoError(t, n2.ResolveNode(dgraph.Unresolvable))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	assert.NoError(t, n3.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.Equals(t, readyNodes[0].ResolutionStatus(), dgraph.Waiting)
}

// Test unresolvable with two ORs and two AND, with an AND being unresolvable.
func TestDirectedGraph_UnresolvableAndsWithOrs(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)
	n4, err := d.AddNode("node-4", "test4")
	assert.NoError(t, err)
	n5, err := d.AddNode("node-5", "test5")
	assert.NoError(t, err)

	// N2 && N3 && (N4 || N5)
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.AndDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.AndDependency))
	assert.NoError(t, n1.ConnectDependency(n4.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n5.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 4)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	assert.SliceContains(t, n4, readyNodes)
	assert.SliceContains(t, n5, readyNodes)

	// Resolve an AND as unresolvable. This should cause instant failure.
	assert.NoError(t, n2.ResolveNode(dgraph.Unresolvable))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.Equals(t, readyNodes[0].ResolutionStatus(), dgraph.Unresolvable)
}

// Test unresolvable with two ORs and two AND, with an OR being unresolvable.
func TestDirectedGraph_UnresolvableOrsWithAnds(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)
	n4, err := d.AddNode("node-4", "test4")
	assert.NoError(t, err)
	n5, err := d.AddNode("node-5", "test5")
	assert.NoError(t, err)

	// N2 && N3 && (N4 || N5)
	assert.NoError(t, n1.ConnectDependency(n2.ID(), dgraph.AndDependency))
	assert.NoError(t, n1.ConnectDependency(n3.ID(), dgraph.AndDependency))
	assert.NoError(t, n1.ConnectDependency(n4.ID(), dgraph.OrDependency))
	assert.NoError(t, n1.ConnectDependency(n5.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 4)
	assert.SliceContains(t, n2, readyNodes)
	assert.SliceContains(t, n3, readyNodes)
	assert.SliceContains(t, n4, readyNodes)
	assert.SliceContains(t, n5, readyNodes)

	// Resolve an AND as unresolvable. This should cause instant failure.
	assert.NoError(t, n4.ResolveNode(dgraph.Unresolvable))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	assert.NoError(t, n5.ResolveNode(dgraph.Unresolvable))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.Equals(t, readyNodes[0].ResolutionStatus(), dgraph.Unresolvable)
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
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	assert.NoError(t, n2.ConnectDependency(n1.ID(), dgraph.AndDependency))
	// Add a connection to ensure no negative effects due to propagation.
	err = n1.ResolveNode(dgraph.Waiting)
	assert.NoError(t, err)
}
