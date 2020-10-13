package transports

import (
	"github.com/vocdoni/multirpc/types"
)

type Transport interface {
	// Init initializes the transport layer. Takes a struct of options. Not all options must have effect.
	Init(c *types.Connection) error
	// ConnectionType returns the human readable name for the transport layer.
	ConnectionType() string
	// Listen starts and keeps listening for new messages, takes a channel where new messages will be written.
	Listen(reciever chan<- types.Message)
	// Send outputs a new message to the connection of MessageContext.
	// The package implementing this interface must also implement its own types.MessageContext.
	Send(msg types.Message) error
	// SendUnicast outputs a new message to a specific connection identified by address.
	SendUnicast(address string, msg types.Message) error
	// AddNamespace creates a new namespace for writing handlers. On HTTP namespace is usually the URL route.
	AddNamespace(namespace string) error
	// Address returns an string containing the transport own address identifier.
	Address() string
	// SetBootnodes takes a list of bootnodes and connects to them for boostraping the network protocol.
	// This function only makes sense on p2p transports.
	SetBootnodes(bootnodes []string)
	// AddPeer opens a new connection with the specified peer.
	// This function only makes sense on p2p transports.
	AddPeer(peer string) error
	// String returns a human readable string representation of the transport state
	String() string
}
