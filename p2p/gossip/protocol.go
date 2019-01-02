package gossip

import (
	"errors"
	"hash/fnv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/smallnest/libp2p/crypto"
	"github.com/smallnest/libp2p/p2p/config"
	"github.com/smallnest/libp2p/p2p/message"
	"github.com/smallnest/libp2p/p2p/node"
	"github.com/smallnest/libp2p/p2p/pb"
	"github.com/smallnest/libp2p/p2p/service"
	"github.com/smallnest/libp2p/log"
)

const messageQBufferSize = 100

const ProtocolName = "/p2p/1.0/gossip"
const protocolVer = "0"

type hash uint32

// fnv.New32 must be used everytime to be sure we get consistent results.
func calcHash(msg []byte) hash {
	msghash := fnv.New32() // todo: Add nonce to messages instead
	msghash.Write(msg)
	return hash(msghash.Sum32())
}

// Interface for the underlying p2p layer
type baseNetwork interface {
	SendMessage(peerPubKey string, protocol string, payload []byte) error
	RegisterProtocol(protocol string) chan service.Message
	SubscribePeerEvents() (conn chan crypto.PublicKey, disc chan crypto.PublicKey)
	ProcessProtocolMessage(sender node.Node, protocol string, data service.Data) error
}

type signer interface {
	PublicKey() crypto.PublicKey
	Sign(data []byte) ([]byte, error)
}

type protocolMessage struct {
	msg *pb.ProtocolMessage
}

// Protocol is the gossip protocol
type Protocol struct {
	config config.SwarmConfig
	net    baseNetwork
	signer signer

	peers    map[string]*peer
	shutdown chan struct{}

	oldMessageMu sync.RWMutex
	oldMessageQ  map[hash]struct{}
	peersMutex   sync.RWMutex

	relayQ   chan service.Message
	messageQ chan protocolMessage
}

// NewProtocol creates a new gossip protocol instance. Call Start to start reading peers
func NewProtocol(config config.SwarmConfig, base baseNetwork, signer signer) *Protocol {
	// intentionally not subscribing to peers events so that the channels won't block in case executing Start delays
	relayChan := base.RegisterProtocol(ProtocolName)
	return &Protocol{
		config:      config,
		net:         base,
		signer:      signer,
		peers:       make(map[string]*peer),
		shutdown:    make(chan struct{}),
		oldMessageQ: make(map[hash]struct{}), // todo : remember to drain this
		peersMutex:  sync.RWMutex{},
		relayQ:      relayChan,
		messageQ:    make(chan protocolMessage, messageQBufferSize),
	}
}

// sender is an interface for peer's p2p layer
type sender interface {
	SendMessage(peerPubKey string, protocol string, payload []byte) error
}

// peer is a struct storing peer's state
type peer struct {
	pubKey        crypto.PublicKey
	msgMutex      sync.RWMutex
	knownMessages map[hash]struct{}
	net           sender
}

func newPeer(net sender, pubKey crypto.PublicKey) *peer {
	return &peer{
		pubKey,
		sync.RWMutex{},
		make(map[hash]struct{}),
		net,
	}
}

// send sends a gossip message to the peer
func (p *peer) send(msg []byte, checksum hash) error {
	// don't do anything if this peer know this msg
	p.msgMutex.RLock()
	if _, ok := p.knownMessages[checksum]; ok {
		p.msgMutex.RUnlock()
		return errors.New("already got this msg")
	}
	p.msgMutex.RUnlock()
	go func() {
		err := p.net.SendMessage(p.pubKey.String(), ProtocolName, msg)
		if err != nil {
			log.Infof("gossip protocol failed to send msg (calcHash %d) to peer %v, first attempt. err=%v", checksum, p.pubKey, err)
			// doing one retry before giving up
			err = p.net.SendMessage(p.pubKey.String(), "", msg)
			if err != nil {
				log.Infof("gossip protocol failed to send msg (calcHash %d) to peer %v, second attempt. err=%v", checksum, p.pubKey, err)
				return
			}
		}
		p.msgMutex.Lock()
		p.knownMessages[checksum] = struct{}{}
		p.msgMutex.Unlock()
	}()
	return nil
}

func (prot *Protocol) Close() {
	close(prot.shutdown)
}

// markMessage adds the calcHash to the old message queue so the message won't be processed in case received again
func (prot *Protocol) markMessage(h hash) {
	prot.oldMessageMu.Lock()
	prot.oldMessageQ[h] = struct{}{}
	prot.oldMessageMu.Unlock()
}

func (prot *Protocol) propagateMessage(msg []byte, h hash) {
	prot.peersMutex.RLock()
	for p := range prot.peers {
		peer := prot.peers[p]
		log.Debugf("sending message to peer %v, hash %d", peer.pubKey, h)
		peer.send(msg, h) // non blocking
	}
	prot.peersMutex.RUnlock()
}

func (prot *Protocol) validateMessage(msg *pb.ProtocolMessage) error {

	err := message.AuthAuthor(msg)

	if err != nil {
		log.Errorf("fail to authorize gossip message, err %v", err)
		return err
	}

	if msg.Metadata.ClientVersion != protocolVer {
		log.Errorf("fail to validate message's protocol version when validating gossip message, err %v", err)
		return err
	}
	return nil
}

// Broadcast is the actual broadcast procedure, loop on peers and add the message to their queues
func (prot *Protocol) Broadcast(payload []byte, nextProt string) error {
	log.Debugf("broadcasting message from type %s", nextProt)
	// add gossip header
	header := &pb.Metadata{
		NextProtocol:  nextProt,
		ClientVersion: protocolVer,
		Timestamp:     time.Now().Unix(),
		AuthPubKey:    prot.signer.PublicKey().Bytes(),
		MsgSign:       nil,
	}

	msg := &pb.ProtocolMessage{
		Metadata: header,
		Data:     &pb.ProtocolMessage_Payload{Payload: payload},
	}

	bin, err := proto.Marshal(msg)
	if err != nil {
		log.Errorf("failed to marshal message when generating gossip header, err %v", err)
		return err

	}

	sign, err2 := prot.signer.Sign(bin)
	if err2 != nil {
		log.Errorf("failed to Sign header when generating gossip header, err %v", err)
		return err
	}

	msg.Metadata.MsgSign = sign

	finbin, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	// so we won't process our own messages
	hash := calcHash(finbin)
	prot.markMessage(hash)
	prot.propagateMessage(finbin, hash)
	return nil
}

// Start a loop that process peers events
func (prot *Protocol) Start() {
	peerConn, peerDisc := prot.net.SubscribePeerEvents() // this was start blocks until we registered.
	go prot.eventLoop(peerConn, peerDisc)
}

func (prot *Protocol) addPeer(peer crypto.PublicKey) {
	prot.peersMutex.Lock()
	prot.peers[peer.String()] = newPeer(prot.net, peer)
	prot.peersMutex.Unlock()
}

func (prot *Protocol) removePeer(peer crypto.PublicKey) {
	prot.peersMutex.Lock()
	delete(prot.peers, peer.String())
	prot.peersMutex.Unlock()
}

func (prot *Protocol) isOldMessage(h hash) bool {
	var oldmessage bool
	prot.oldMessageMu.RLock()
	if _, ok := prot.oldMessageQ[h]; ok {
		oldmessage = true
	} else {
		oldmessage = false
	}
	prot.oldMessageMu.RUnlock()
	return oldmessage
}

func (prot *Protocol) handleRelayMessage(msgB []byte) error {
	hash := calcHash(msgB)

	// in case the message was received through the relay channel we need to remove the Gossip layer and hand the payload for the next protocol to process
	if prot.isOldMessage(hash) {
		// todo : - have some more metrics for termination
		// todo	: - maybe tell the peer weg ot this message already?
		log.Debugf("got old message, hash %d", hash)
	} else {

		msg := &pb.ProtocolMessage{}
		err := proto.Unmarshal(msgB, msg)
		if err != nil {
			log.Errorf("failed to unmarshal when handling relay message, err %v", err)
			return err
		}

		err = prot.validateMessage(msg)
		if err != nil {
			log.Errorf("failed to validate message when handling relay message, err %v", err)
			return err
		}

		prot.markMessage(hash)

		var data service.Data

		if payload := msg.GetPayload(); payload != nil {
			data = service.Data_Bytes{Payload: payload}
		} else if wrap := msg.GetMsg(); wrap != nil {
			data = service.Data_MsgWrapper{Req: wrap.Req, MsgType: wrap.Type, ReqID: wrap.ReqID, Payload: wrap.Payload}
		}

		authKey, err := crypto.NewPublicKey(msg.Metadata.AuthPubKey)
		if err != nil {
			log.Errorf("failed to decode the auth public key when handling relay message, err %v", err)
			return err
		}

		go prot.net.ProcessProtocolMessage(node.New(authKey, ""), msg.Metadata.NextProtocol, data)
	}

	prot.propagateMessage(msgB, hash)

	return nil
}

func (prot *Protocol) eventLoop(peerConn chan crypto.PublicKey, peerDisc chan crypto.PublicKey) {
	var err error
loop:
	for {
		select {
		case msg, ok := <-prot.relayQ:
			if !ok {
				err = errors.New("channel closed")
				break loop
			}
			// incoming messages from p2p layer for process and relay
			go func() {
				//  [todo some err handling
				prot.handleRelayMessage(msg.Bytes())
			}()
		case peer := <-peerConn:
			go prot.addPeer(peer)
		case peer := <-peerDisc:
			go prot.removePeer(peer)
		case <-prot.shutdown:
			err = errors.New("protocol shutdown")
			break loop
		}
	}
	log.Warnf("gossip protocol event loop stopped. err: %v", err)
}

// peersCount returns the number of peers know to the protocol, used for testing only
func (prot *Protocol) peersCount() int {
	prot.peersMutex.RLock()
	cnt := len(prot.peers)
	prot.peersMutex.RUnlock()
	return cnt
}

// hasPeer returns whether or not a peer is known to the protocol, used for testing only
func (prot *Protocol) hasPeer(key crypto.PublicKey) bool {
	prot.peersMutex.RLock()
	_, ok := prot.peers[key.String()]
	prot.peersMutex.RUnlock()
	return ok
}
