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

func TestDirectedGraph_OneAndDependency1(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)

	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)

	// Use simple connect method, which will set an AND dependency.
	assert.NoError(t, n1.Connect(n2.ID()))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.NoError(t, n1.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n2.ID())
}

func TestDirectedGraph_OneAndDependency2(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)

	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)

	// Use the dependency connection method instead.
	assert.NoError(t, n2.ConnectDependency(n1.ID(), dgraph.AndDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.NoError(t, n1.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n2.ID())
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
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)

	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)

	// Use the dependency connection method instead.
	assert.NoError(t, n2.ConnectDependency(n1.ID(), dgraph.OrDependency))

	assert.NoError(t, d.PushStartingNodes())
	readyNodes := d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
	assert.NoError(t, n1.ResolveNode(dgraph.Resolved))
	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n2.ID())
}

// Test two OR dependencies.
func TestDirectedGraph_TwoOrDependencies1(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	// Use the dependency connection method instead.
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

func TestDirectedGraph_TwoOrDependencies2(t *testing.T) {
	d := dgraph.New[string]()
	n1, err := d.AddNode("node-1", "test1")
	assert.NoError(t, err)
	n2, err := d.AddNode("node-2", "test2")
	assert.NoError(t, err)
	n3, err := d.AddNode("node-3", "test3")
	assert.NoError(t, err)

	// Use the dependency connection method instead.
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

// Test two ANDs and two OR dependencies, with the OR resolving before the AND.
func TestDirectedGraph_TwoOrAndAndDependencies1(t *testing.T) {
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

	// Use the dependency connection method instead.
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
	// Resolve one OR. That alone is not enough for n1 to be ready. No need to resolve n2, too.
	assert.NoError(t, n2.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	// Resolve both ANDs. It should resolve once both of them are resolved, but not just one.
	assert.NoError(t, n4.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	assert.NoError(t, n5.ResolveNode(dgraph.Resolved))

	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
}

// Test two ANDs and two OR dependencies, with the AND resolving before the OR.
func TestDirectedGraph_TwoOrAndAndDependencies2(t *testing.T) {
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

	// Use the dependency connection method instead.
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

	// Resolve both ANDs. It still needs the OR to resolve.
	assert.NoError(t, n4.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)
	assert.NoError(t, n5.ResolveNode(dgraph.Resolved))
	assert.Equals(t, len(d.PopReadyNodes()), 0)

	// Resolve one OR. That should now be enough to resolve it.
	assert.NoError(t, n2.ResolveNode(dgraph.Resolved))

	readyNodes = d.PopReadyNodes()
	assert.Equals(t, len(readyNodes), 1)
	assert.Equals(t, readyNodes[0].ID(), n1.ID())
}

// TODO: Test unresolvable with a simple AND
// TODO: Test unresolvable with a simple OR.
// TODO: Test unresolvable with two AND with one failure.
// TODO: Test unresolvable with two OR with one failure and one success.
// TODO: Test unresolvable with a simple completion dependency.
// TODO: Test unresolvable with a completion dependency and an AND.
// TODO: Test unresolvable with a completion dependency and two OR, with the completion dependency unresolvable.
// TODO: Test unresolvable with two ORs and two AND.
