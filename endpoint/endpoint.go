package endpoint

import "github.com/vocdoni/multirpc/transports"

// Endpoint represents a valid Endpoint for the multirpc stack
type Endpoint interface {
	Init(config map[string]string) error
	SetOption(name string, value interface{}) error
	Transport() transports.Transport
	ID() string
}
