package dht

import (
	"testing"

	"github.com/smallnest/libp2p/p2p/config"
	"github.com/smallnest/libp2p/p2p/node"
	"github.com/smallnest/libp2p/p2p/service"
	"github.com/stretchr/testify/assert"
)

func TestFindNodeProtocol_FindNode(t *testing.T) {

	cfg := config.DefaultConfig()
	sim := service.NewSimulator()

	n1 := sim.NewNode()
	rt1 := NewRoutingTable(cfg.SwarmConfig.RoutingTableBucketSize, n1.DhtID())
	fnd1 := newFindNodeProtocol(n1, rt1)

	n2 := sim.NewNode()
	rt2 := NewRoutingTable(cfg.SwarmConfig.RoutingTableBucketSize, n2.DhtID())
	_ = newFindNodeProtocol(n2, rt2)

	idarr, err := fnd1.FindNode(n2.Node, node.GenerateRandomNodeData().String())

	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, []node.Node{}, idarr, "Should be an empty array")
}

func TestFindNodeProtocol_FindNode2(t *testing.T) {
	randnode := node.GenerateRandomNodeData()

	cfg := config.DefaultConfig()
	sim := service.NewSimulator()

	n1 := sim.NewNode()
	rt1 := NewRoutingTable(cfg.SwarmConfig.RoutingTableBucketSize, n1.DhtID())
	fnd1 := newFindNodeProtocol(n1, rt1)

	n2 := sim.NewNode()
	rt2 := NewRoutingTable(cfg.SwarmConfig.RoutingTableBucketSize, n2.DhtID())
	fnd2 := newFindNodeProtocol(n2, rt2)

	fnd2.rt.Update(randnode)

	idarr, err := fnd1.FindNode(n2.Node, randnode.String())

	expected := []node.Node{randnode}

	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, expected, idarr, "Should be array that contains the node")

	for _, n := range node.GenerateRandomNodesData(10) {
		fnd2.rt.Update(n)
		expected = append(expected, n)
	}

	// sort because this is how its returned
	expected = node.SortByDhtID(expected, randnode.DhtID())

	idarr, err = fnd1.FindNode(n2.Node, randnode.String())

	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, expected, idarr, "Should be same array")

	idarr, err = fnd2.FindNode(n1.Node, randnode.String())

	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, expected, idarr, "Should be array that contains the node")
}
