package net

import (
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/smallnest/libp2p/crypto"
	"gopkg.in/op/go-logging.v1"
)

// ReadWriteCloserMock is a mock of ReadWriteCloserMock
type ReadWriteCloserMock struct {
}

// Read reads something
func (m ReadWriteCloserMock) Read(p []byte) (n int, err error) {
	return 0, nil
}

// Write mocks write
func (m ReadWriteCloserMock) Write(p []byte) (n int, err error) {
	return 0, nil
}

// Close mocks close
func (m ReadWriteCloserMock) Close() error {
	return nil
}

//RemoteAddr mocks remote addr return
func (m ReadWriteCloserMock) RemoteAddr() net.Addr {
	r, err := net.ResolveTCPAddr("tcp", "127.0.0.0")
	if err != nil {
		panic(err)
	}
	return r
}

// NetworkMock is a mock struct
type NetworkMock struct {
	dialErr          error
	dialDelayMs      int8
	dialCount        int32
	preSessionErr    error
	preSessionCount  int32
	regNewRemoteConn []chan NewConnectionEvent
	networkId        int8
	closingConn      []chan Connection
	incomingMessages []chan IncomingMessageEvent
	dialSessionID    []byte
	logger           *logging.Logger
}

// NewNetworkMock is a mock
func NewNetworkMock() *NetworkMock {
	return &NetworkMock{
		regNewRemoteConn: make([]chan NewConnectionEvent, 0),
		closingConn:      make([]chan Connection, 0),
		incomingMessages: []chan IncomingMessageEvent{make(chan IncomingMessageEvent, 256)},
	}
}

func (n *NetworkMock) reset() {
	n.dialCount = 0
	n.dialDelayMs = 0
	n.dialErr = nil
}

func (n *NetworkMock) SetNextDialSessionID(sID []byte) {
	n.dialSessionID = sID
}

// SetDialResult is a mock
func (n *NetworkMock) SetDialResult(err error) {
	n.dialErr = err
}

// SetDialDelayMs sets delay
func (n *NetworkMock) SetDialDelayMs(delay int8) {
	n.dialDelayMs = delay
}

// Dial dials
func (n *NetworkMock) Dial(address string, remotePublicKey crypto.PublicKey) (Connection, error) {
	atomic.AddInt32(&n.dialCount, 1)
	time.Sleep(time.Duration(n.dialDelayMs) * time.Millisecond)
	sID := n.dialSessionID
	if sID == nil {
		sID = make([]byte, 4)
		rand.Read(sID)
	}
	conn := NewConnectionMock(remotePublicKey)
	conn.SetSession(SessionMock{id: sID})
	return conn, n.dialErr
}

// DialCount gets the dial count
func (n *NetworkMock) DialCount() int32 {
	return atomic.LoadInt32(&n.dialCount)
}

// SubscribeOnNewRemoteConnections subscribes on new connections
func (n *NetworkMock) SubscribeOnNewRemoteConnections() chan NewConnectionEvent {
	ch := make(chan NewConnectionEvent, 20)
	n.regNewRemoteConn = append(n.regNewRemoteConn, ch)
	return ch
}

// PublishNewRemoteConnection and stuff
func (n NetworkMock) PublishNewRemoteConnection(nce NewConnectionEvent) {
	for _, ch := range n.regNewRemoteConn {
		ch <- nce
	}
}

// SubscribeClosingConnections subscribes on new connections
func (n *NetworkMock) SubscribeClosingConnections() chan Connection {
	ch := make(chan Connection, 20)
	n.closingConn = append(n.closingConn, ch)
	return ch
}

// publishClosingConnection and stuff
func (n NetworkMock) publishClosingConnection(con Connection) {
	for _, ch := range n.closingConn {
		ch <- con
	}
}

// PublishClosingConnection is a hack to expose the above method in the mock but still impl the same interface
func (n NetworkMock) PublishClosingConnection(con Connection) {
	n.publishClosingConnection(con)
}

func (n *NetworkMock) setNetworkId(id int8) {
	n.networkId = id
}

// NetworkID is netid
func (n *NetworkMock) NetworkID() int8 {
	return n.networkId
}

// IncomingMessages return channel of IncomingMessages
func (n *NetworkMock) IncomingMessages() []chan IncomingMessageEvent {
	return n.incomingMessages
}

// EnqueueMessage return channel of IncomingMessages
func (n *NetworkMock) EnqueueMessage(event IncomingMessageEvent) {
	n.incomingMessages[0] <- event
}

// SetPreSessionResult does this
func (n *NetworkMock) SetPreSessionResult(err error) {
	n.preSessionErr = err
}

// PreSessionCount counts
func (n NetworkMock) PreSessionCount() int32 {
	return atomic.LoadInt32(&n.preSessionCount)
}

// HandlePreSessionIncomingMessage and stuff
func (n *NetworkMock) HandlePreSessionIncomingMessage(c Connection, msg []byte) error {
	atomic.AddInt32(&n.preSessionCount, 1)
	return n.preSessionErr
}

// Logger return the logger
func (n *NetworkMock) Logger() *logging.Logger {
	return n.logger
}
