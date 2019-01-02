package message

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/smallnest/libp2p/crypto"
	"github.com/smallnest/libp2p/p2p/config"
	"github.com/smallnest/libp2p/p2p/net"
	"github.com/smallnest/libp2p/p2p/pb"
)

// PrepareMessage prepares a message for sending on a given session, session must be checked first
func PrepareMessage(ns net.NetworkSession, data []byte) ([]byte, error) {
	encPayload, err := ns.Encrypt(data)
	if err != nil {
		return nil, fmt.Errorf("aborting send - failed to encrypt payload: %v", err)
	}

	return encPayload, nil
}

// NewProtocolMessageMetadata creates meta-data for an outgoing protocol message authored by this node.
func NewProtocolMessageMetadata(author crypto.PublicKey, protocol string) *pb.Metadata {
	return &pb.Metadata{
		NextProtocol:  protocol,
		ClientVersion: config.ClientVersion,
		Timestamp:     time.Now().Unix(),
		AuthPubKey:    author.Bytes(),
	}
}

// SignMessage signs a message with a privatekey.
func SignMessage(pv crypto.PrivateKey, pm *pb.ProtocolMessage) error {
	data, err := proto.Marshal(pm)
	if err != nil {
		e := fmt.Errorf("invalid msg format %v", err)
		return e
	}

	sign, err := pv.Sign(data)
	if err != nil {
		return fmt.Errorf("failed to sign message err:%v", err)
	}

	pm.Metadata.MsgSign = sign

	return nil
}

// AuthAuthor authorizes that a message is signed by its claimed author
func AuthAuthor(pm *pb.ProtocolMessage) error {
	if pm == nil || pm.Metadata == nil {
		return fmt.Errorf("can't sign defected message, message or metadata was empty")
	}

	sign := pm.Metadata.MsgSign
	sPubkey := pm.Metadata.AuthPubKey

	pubkey, err := crypto.NewPublicKey(sPubkey)
	if err != nil {
		return fmt.Errorf("could'nt create public key from %v, err: %v", hex.EncodeToString(sPubkey), err)
	}

	pm.Metadata.MsgSign = nil // we have to verify the message without the sign

	bin, err := proto.Marshal(pm)

	if err != nil {
		return err
	}

	v, err := pubkey.Verify(bin, sign)

	if err != nil {
		return err
	}

	if !v {
		return fmt.Errorf("coudld'nt verify message")
	}

	pm.Metadata.MsgSign = sign // restore sign because maybe we'll send it again ( gossip )

	return nil
}
