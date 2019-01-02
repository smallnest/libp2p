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

// NewNodeIdentity creates a new local node without attempting to restore node from local store.
func NewNodeIdentity(config config.Config, address string) (*LocalNode, error) {
	var priv crypto.PrivateKey
	var pub crypto.PublicKey
	var err error
	if config.PrivateKey == "" {
		priv, pub, err = crypto.GenerateKeyPair()
		if err != nil {
			return nil, err
		}
	} else {
		priv, err = crypto.NewPrivateKeyFromString(config.PrivateKey)
		if err != nil {
			return nil, err
		}
	}

	return newLocalNodeWithKeys(pub, priv, address, config.NetworkID)
}

func newLocalNodeWithKeys(pubKey crypto.PublicKey, privKey crypto.PrivateKey, address string, networkID int8) (*LocalNode, error) {

	n := &LocalNode{
		Node: Node{
			pubKey:  pubKey,
			address: address,
		},
		networkID: networkID,
		privKey:   privKey,
	}

	log.Infof("local node identity >> %v", n.String())
	return n, nil
}
