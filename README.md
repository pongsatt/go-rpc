Go RPC (using messaging system)
================================

Go library for RPC client/server based on messaging system (currently support only Kafka)

[![Test](https://github.com/pongsatt/go-rpc/actions/workflows/test.yml/badge.svg)](https://github.com/pongsatt/go-rpc/actions/workflows/test.yml)

Features include:

  * [RPC on kafka](#running-example)
  * [Code generation for client proxy and rpc server](#code-generation)

Running example
-------------------------------------------------------------------------------------------

Server Example

```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pongsatt/go-rpc"
	"github.com/pongsatt/go-rpc/example"
	"github.com/pongsatt/go-rpc/messaging"
)

func main() {
	kafkaClient := messaging.NewCpKafkaClient(&messaging.CpKafkaConfig{
		Brokers: "localhost:9092",
	})

	requestReplyClient := rpc.NewRequestReplyClient("RealServer", kafkaClient, &rpc.RequestReplyConfig{})

	err := example.NewRealServerProvider(
		requestReplyClient, &example.RealServer{})
	if err != nil {
		panic(err)
	}
	fmt.Println("server started")

	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)

	<-termChan
	kafkaClient.Shutdown()
}

```
Start kafka server (using Readpanda for simplicity)
```sh
./start_servers.sh
```

Start server
```sh
go run example/server/*
```

Server output
```console
➜  go-rpc go run example/server/*
server started
```

Client Example

```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pongsatt/go-rpc"
	"github.com/pongsatt/go-rpc/example"
	"github.com/pongsatt/go-rpc/messaging"
)

func main() {
	kafkaClient := messaging.NewCpKafkaClient(&messaging.CpKafkaConfig{
		Brokers: "localhost:9092",
	})

	timeout := 30 * time.Second
	requestReplyClient := rpc.NewRequestReplyClient("RealServer", kafkaClient, &rpc.RequestReplyConfig{
		Timeout: &timeout,
	})

	// interface -> proxy
	proxy := example.NewServerProxy(requestReplyClient)

	// execute normal code
	client := example.NewCleint(proxy)

	for i := 0; i < 100; i++ {
		start := time.Now()
		id, err := client.Create("test")
		fmt.Printf("time use %s\n", time.Since(start))

		if err != nil {
			fmt.Printf("error creating %v\n", err)
		} else {
			fmt.Printf("got id %s\n", id)
		}
	}

	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)

	<-termChan
	kafkaClient.Shutdown()
}
```

Run client
```sh
go run example/client/*
```

Client output
```console
➜  go-rpc go run example/client/*
time use 275.060096ms
got id Hi test
```

Code Generation
-------------------------------------------------------------------------------------------

Server
```go
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
```
Client
```go
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
```

Run go generate
```sh
go generate ./...
```

Output
```console
generating proxy
writing file client_gen.go # proxy for Server interface
generate done
generating provider
writing file realserver_gen.go # run rpc server for RealServer struct
generate done
```