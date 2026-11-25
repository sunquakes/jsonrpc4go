package main

import (
	"flag"
	"strings"

	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/server"
)

type IntRpc struct{}

type Result struct {
	C int `json:"c"`
}

func (i *IntRpc) Add(a, b int) int {
	return a + b
}

func (i *IntRpc) Add2(a, b int) Result {
	return Result{C: a + b}
}

type StreamRpc struct{}

// Count returns a slice, so the streamable HTTP transport pushes each value
// as an SSE event.
func (s *StreamRpc) Count(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func main() {
	port := flag.Int("port", 3232, "listen port")
	origins := flag.String("origins", "", "comma separated allowed browser origins (empty allows all)")
	hosts := flag.String("hosts", "", "comma separated allowed Host header values (empty allows all; set on localhost-only servers)")
	flag.Parse()
	s, _ := jsonrpc4go.NewServer("streamable-http", *port)
	options := server.StreamableHttpOptions{}
	if *origins != "" {
		options.AllowedOrigins = strings.Split(*origins, ",")
	}
	if *hosts != "" {
		options.AllowedHosts = strings.Split(*hosts, ",")
	}
	if *origins != "" || *hosts != "" {
		s.SetOptions(options)
	}
	s.Register(new(IntRpc))
	s.Register(new(StreamRpc))
	// Fixture tools/resources/prompts for the MCP conformance suite.
	if hs, ok := s.(*server.StreamableHttpServer); ok {
		registerFixtures(hs)
	}
	s.Start() // serves /mcp for both JSON-RPC and MCP clients
}
