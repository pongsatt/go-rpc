//go:generate go run github.com/pongsatt/go-rpc/cmd/gen proxy

package example

// Server interface
type Server interface {
	GetID(seed string) (string, error)
	Create(order *Order) (string, error)
}

// Client struct
type Client struct {
	server Server
}

// NewCleint new instance
func NewCleint(server Server) *Client {
	return &Client{server}
}

// Create func
func (client *Client) Create(seed string) (string, error) {
	return client.server.GetID(seed)
}
