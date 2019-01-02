package node

import (
	"github.com/smallnest/libp2p/crypto"
	"github.com/smallnest/libp2p/p2p/config"
	"github.com/smallnest/log"
)

// LocalNode implementation.
type LocalNode struct {
	Node
	privKey crypto.PrivateKey

	networkID int8
}

// NetworkID returns the local node's network id (testnet/mainnet, etc..)
func (n *LocalNode) NetworkID() int8 {
	return n.networkID
}

// PrivateKey returns this node's private key.
func (n *LocalNode) PrivateKey() crypto.PrivateKey {
	return n.privKey
}

// NewLocalNode creates a local node with a provided ip address.
// Attempts to set node node from persisted data in local store.
// Creates a new node if none was loaded.
func NewLocalNode(config config.Config, address string, persist bool) (*LocalNode, error) {

	if len(config.NodeID) > 0 {
		// user provided node id/pubkey via the cli - attempt to start that node w persisted data
		data, err := readNodeData(config.NodeID)
		if err != nil {
			return nil, err
		}

		return newLocalNodeFromFile(address, data, persist)
	}

	// look for persisted node data in the nodes directory
	// load the node with the data of the first node found
	nodeData, err := readFirstNodeData()
	if err != nil {
		log.Warning("failed to read node data from local store")
	}

	if nodeData != nil {
		// create node using persisted node data
		return newLocalNodeFromFile(address, nodeData, persist)
	}

	// generate new node
	return NewNodeIdentity(config, address, persist)
}

// NewNodeIdentity creates a new local node without attempting to restore node from local store.
func NewNodeIdentity(config config.Config, address string, persist bool) (*LocalNode, error) {
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	return newLocalNodeWithKeys(pub, priv, address, config.NetworkID, persist)
}

func newLocalNodeWithKeys(pubKey crypto.PublicKey, privKey crypto.PrivateKey, address string, networkID int8, persist bool) (*LocalNode, error) {

	n := &LocalNode{
		Node: Node{
			pubKey:  pubKey,
			address: address,
		},
		networkID: networkID,
		privKey:   privKey,
	}

	log.Info("Local node identity >> %v", n.String())

	if persist {
		// persist store data so we can start it on future app sessions
		err := n.persistData()
		if err != nil { // no much use of starting if we can't store node private key in store
			log.Error("failed to persist node data to local store", err)
			return nil, err
		}
	}

	return n, nil
}

// Creates a new node from persisted NodeData.
func newLocalNodeFromFile(address string, d *nodeFileData, persist bool) (*LocalNode, error) {

	priv, err := crypto.NewPrivateKeyFromString(d.PrivKey)
	if err != nil {
		return nil, err
	}

	pub, err := crypto.NewPublicKeyFromString(d.PubKey)
	if err != nil {
		return nil, err
	}

	log.Info(">>>> Creating node identity from filesystem existing key %s", pub.String())

	return newLocalNodeWithKeys(pub, priv, address, d.NetworkID, persist)
}
