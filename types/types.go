package types

import (
	"encoding/json"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
)

type MessageContext interface {
	ConnectionType() string
	Send(Message) error
}

// Message is a wrapper for messages from various net transport modules
type Message struct {
	Data      []byte
	TimeStamp int32
	Namespace string

	Context MessageContext
}

// Connection describes the settings for any of the transports defined in the net module, note that not all
// fields are used for all transport types.
type Connection struct {
	Topic        string // channel/topic for topic based messaging such as PubSub
	Encryption   string // what type of encryption to use
	TransportKey string // transport layer key for encrypting messages
	Key          string // this node's key
	Address      string // this node's address
	TLSdomain    string // tls domain
	TLScertDir   string // tls certificates directory
	Port         int32  // specific port on which a transport should listen
}

type MessageAPI interface {
	GetID() string
	SetID(string)
	SetTimestamp(int32)
	SetError(string)
	GetMethod() string
}

type RouterRequest struct {
	MessageContext
	Message            MessageAPI
	Method             string
	Id                 string
	Authenticated      bool
	Address            ethcommon.Address
	SignaturePublicKey string
	Private            bool
	Signer             *ethereum.SignKeys
}

type RequestMessage struct {
	MessageAPI json.RawMessage `json:"request"`

	ID        string `json:"id"`
	Signature string `json:"signature"`
}

type HTTPapi struct {
	ListenHost string
	ListenPort int32
	TLSdomain  string
	TLSdirCert string
	Metrics    *Metrics
}

// MetricsCfg initializes the metrics config
type Metrics struct {
	Enabled         bool
	RefreshInterval int
}
