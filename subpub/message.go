package subpub

import (
	"bufio"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/vocdoni/multirpc/transports"
	"go.vocdoni.io/dvote/log"
)

type MessageContext struct {
	Message []byte
	Stream  network.Stream
}

func (mc *MessageContext) ConnectionType() string {
	return "subpub"
}

func (mc *MessageContext) Send(m transports.Message) error {
	_, err := mc.Stream.Write(m.Data)
	return err
}

// SendMessage encrypts and writes a message on the readwriter buffer
func (ps *SubPub) SendMessage(w *bufio.Writer, msg []byte) error {
	log.Debugf("sending message: %s", msg)
	if !ps.Private {
		msg = []byte(ps.encrypt(msg)) // TO-DO find a better way to encapsulate data! byte -> b64 -> byte is not the best...
	}
	if _, err := w.Write(append(msg, byte(delimiter))); err != nil {
		return err
	}
	return w.Flush()
}
