package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/vocdoni/multirpc/example/subpub/message"
	"github.com/vocdoni/multirpc/router"
	"github.com/vocdoni/multirpc/transports"
	"github.com/vocdoni/multirpc/transports/subpubtransport"
	"go.vocdoni.io/dvote/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"

	"go.vocdoni.io/dvote/log"
)

var sharedKey = "sharedSecret123"

func processLine(input []byte) *message.MyAPI {
	var req message.MyAPI
	err := json.Unmarshal(input, &req)
	if err != nil {
		panic(err)
	}
	return &req
}

func main() {
	logLevel := flag.String("logLevel", "error", "log level <debug, info, warn, error>")
	privKey := flag.String("key", "", "private key for signature (leave blank for auto-generate)")
	flag.Parse()
	log.Init(*logLevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	// Set or generate client signing key
	signer := ethereum.NewSignKeys()
	if *privKey != "" {
		if err := signer.AddHexKey(*privKey); err != nil {
			panic(err)
		}
	} else {
		signer.Generate()
	}
	_, priv := signer.HexString()
	conn := transports.Connection{
		Port:         7799,
		Key:          priv,
		Topic:        fmt.Sprintf("%x", ethereum.HashRaw([]byte(sharedKey))),
		TransportKey: sharedKey,
	}
	sp := subpubtransport.SubPubHandle{}
	//sp := subpub.NewSubPub(signer.Private,ethereum.HashRaw([]byte(sharedKey)), 7799, false)
	//sp.Start()

	if err := sp.Init(&conn); err != nil {
		log.Fatal(err)
	}
	msg := make(chan transports.Message)
	sp.Listen(msg)

	log.Info("UNIX socket created at /tmp/subpub.sock")
	l, err := net.Listen("unix", "/tmp/subpub.sock")
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer os.Remove("/tmp/subpub.sock")

	var req *message.MyAPI
	read := func(close chan (bool), fd net.Conn) {
		log.Infof("listening to subpub and writing to the unix socket")
		for {
			select {
			case data := <-msg:
				fmt.Printf("Peer:%s %s", data.Context.(*subpubtransport.SubPubContext).PeerID, data.Data)
				fd.Write(data.Data)
			case <-close:
				log.Warnf("EOF")
				return
			}
		}
	}

	for {
		fd, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}
		close := make(chan (bool), 1)
		go read(close, fd)
		reader := bufio.NewReader(fd)
		for {
			line, _, err := reader.ReadLine()
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
			if len(line) < 7 || strings.HasPrefix(string(line), "#") {
				continue
			}
			req = processLine(line)
			if err := sp.Send(transports.Message{Data: Request(req, signer), Namespace: "/main"}); err != nil {
				log.Error(err)
				continue
			}
		}
		close <- true
	}
}

func Request(req *message.MyAPI, signer *ethereum.SignKeys) []byte {
	// Prepare and send request
	req.Timestamp = (int32(time.Now().Unix()))
	req.ID = fmt.Sprintf("%d", rand.Intn(1000))
	reqInner, err := crypto.SortedMarshalJSON(req)
	if err != nil {
		log.Fatalf("%s: %v", req.Method, err)
	}
	var signature []byte
	if signer != nil {
		signature, err = signer.Sign(reqInner)
		if err != nil {
			log.Fatalf("%s: %v", req.Method, err)
		}
	}

	reqOuter := router.RequestMessage{
		ID:         req.ID,
		Signature:  signature,
		MessageAPI: reqInner,
	}
	reqBody, err := json.Marshal(reqOuter)
	if err != nil {
		log.Fatalf("%s: %v", req.Method, err)
	}
	return reqBody

}
