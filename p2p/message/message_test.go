package message

import (
	"github.com/gogo/protobuf/proto"
	"github.com/smallnest/libp2p/crypto"
	"github.com/smallnest/libp2p/p2p/config"
	"github.com/smallnest/libp2p/p2p/node"
	"github.com/smallnest/libp2p/p2p/pb"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_NewProtocolMessageMeatadata(t *testing.T) {
	//newProtocolMessageMetadata()
	_, pk, _ := crypto.GenerateKeyPair()
	const gossip = false

	assert.NotNil(t, pk)

	meta := NewProtocolMessageMetadata(pk, "EX")

	assert.NotNil(t, meta, "should be a metadata")
	assert.Equal(t, meta.Timestamp, time.Now().Unix())
	assert.Equal(t, meta.ClientVersion, config.ClientVersion)
	assert.Equal(t, meta.AuthPubKey, pk.Bytes())
	assert.Equal(t, meta.NextProtocol, "EX")
	assert.Equal(t, meta.MsgSign, []byte(nil))
}

func TestSwarm_AuthAuthor(t *testing.T) {
	// create a message

	priv, pub, err := crypto.GenerateKeyPair()

	assert.NoError(t, err, err)
	assert.NotNil(t, priv)
	assert.NotNil(t, pub)

	pm := &pb.ProtocolMessage{
		Metadata: NewProtocolMessageMetadata(pub, "EX"),
		Data:     &pb.ProtocolMessage_Payload{[]byte("EX")},
	}
	ppm, err := proto.Marshal(pm)
	assert.NoError(t, err, "cant marshal msg ", err)

	// sign it
	s, err := priv.Sign(ppm)
	assert.NoError(t, err, "cant sign ", err)

	pm.Metadata.MsgSign = s

	vererr := AuthAuthor(pm)
	assert.NoError(t, vererr)

	priv2, pub2, err := crypto.GenerateKeyPair()

	assert.NoError(t, err, err)
	assert.NotNil(t, priv2)
	assert.NotNil(t, pub2)

	s, err = priv2.Sign(ppm)
	assert.NoError(t, err, "cant sign ", err)

	pm.Metadata.MsgSign = s

	vererr = AuthAuthor(pm)
	assert.Error(t, vererr)
}

func TestSwarm_SignAuth(t *testing.T) {
	n, _ := node.GenerateTestNode(t)
	pm := &pb.ProtocolMessage{
		Metadata: NewProtocolMessageMetadata(n.PublicKey(), "EX"),
		Data:     &pb.ProtocolMessage_Payload{[]byte("EX")},
	}

	err := SignMessage(n.PrivateKey(), pm)
	assert.NoError(t, err)

	err = AuthAuthor(pm)

	assert.NoError(t, err)
}
