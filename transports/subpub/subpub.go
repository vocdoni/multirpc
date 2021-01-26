package subpubtransport

import (
	"context"
	"fmt"
	"time"

	"github.com/vocdoni/multirpc/subpub"
	"github.com/vocdoni/multirpc/transports"
	"go.vocdoni.io/dvote/crypto/ethereum"
)

type SubPubHandle struct {
	Conn      *transports.Connection
	SubPub    *subpub.SubPub
	BootNodes []string
}

func (p *SubPubHandle) Init(c *transports.Connection) error {
	p.Conn = c
	s := ethereum.NewSignKeys()
	if err := s.AddHexKey(p.Conn.Key); err != nil {
		return fmt.Errorf("cannot import privkey %s: %s", p.Conn.Key, err)
	}
	if len(p.Conn.TransportKey) == 0 {
		return fmt.Errorf("groupkey topic not specified")
	}
	if p.Conn.Port == 0 {
		p.Conn.Port = 45678
	}
	private := p.Conn.Encryption == "private"
	sp := subpub.NewSubPub(s.Private, []byte(p.Conn.TransportKey), p.Conn.Port, private)
	c.Address = sp.PubKey
	p.SubPub = sp
	return nil
}

func (s *SubPubHandle) Listen(reciever chan<- transports.Message) {
	ctx := context.TODO()
	s.SubPub.Start(ctx)
	go s.SubPub.Subscribe(ctx)
	var msg transports.Message
	var msgctx subpub.MessageContext
	for {
		msgctx = <-s.SubPub.Reader
		msg.Data = msgctx.Message
		msg.TimeStamp = int32(time.Now().Unix())
		msg.Context = &msgctx
		reciever <- msg
	}
}

func (s *SubPubHandle) Address() string {
	return s.SubPub.NodeID
}

func (s *SubPubHandle) SetBootnodes(bootnodes []string) {
	s.SubPub.BootNodes = bootnodes
}

func (s *SubPubHandle) AddPeer(peer string) error {
	return s.SubPub.TransportConnectPeer(peer)
}

func (s *SubPubHandle) String() string {
	return s.SubPub.String()
}

func (s *SubPubHandle) ConnectionType() string {
	return "SubPub"
}

func (s *SubPubHandle) Send(msg transports.Message) error {
	if msg.Context != nil {
		return msg.Context.Send(msg)
	}
	s.SubPub.BroadcastWriter <- msg.Data
	return nil
}

func (s *SubPubHandle) AddNamespace(namespace string) error {
	// TBD (could subscrive to a specific topic)
	return nil
}

func (s *SubPubHandle) SendUnicast(address string, msg transports.Message) error {
	// TBD: check if send unicast is really needed, maybe with Send() and the MessageContext the same can be achieved
	if err := s.SubPub.PeerStreamWrite(address, msg.Data); err != nil {
		return fmt.Errorf("cannot send message to %s: (%s)", address, err)
	}
	return nil
}
