//go:generate go run github.com/pongsatt/go-rpc/cmd/gen provider -name=RealServer

package example

// RealServer struct
type RealServer struct {
}

// NewRealServer creates new instance
func NewRealServer() *RealServer {
	return &RealServer{}
}

// GetID func
func (proxy *RealServer) GetID(seed string) (string, error) {
	return "Hi " + seed, nil
}

// Create order
func (proxy *RealServer) Create(order *Order) (string, error) {
	return "ok", nil
}
