package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"golang.org/x/time/rate"
)

const (
	McpEndpoint          = "/mcp"
	McpSessionHeader     = "Mcp-Session-Id"
	McpProtocolHeader    = "Mcp-Protocol-Version"
	McpLastEventHeader   = "Last-Event-ID"
	mcpEventRingSize     = 128
	mcpServerInfoName    = "jsonrpc4go"
	mcpServerInfoVersion = "1.0.0"
)

// mcpSupportedVersions lists the MCP protocol versions this server accepts.
var mcpSupportedVersions = []string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25"}

const mcpLatestVersion = "2025-11-25"

/*
 * StreamableHttp represents the streamable HTTP protocol implementation.
 * @property Port - The port to listen on
 */
type StreamableHttp struct {
	Port int
}

/*
 * StreamableHttpOptions represents the options for the streamable HTTP server
 * @property AllowedOrigins - Browser origins accepted in the Origin header;
 * an empty list allows every origin
 * @property AllowedHosts - Host names accepted in the Host header (with or
 * without port); an empty list allows every host. Set it on localhost-only
 * servers to prevent DNS rebinding attacks.
 */
type StreamableHttpOptions struct {
	AllowedOrigins []string
	AllowedHosts   []string
}

/*
 * sseEvent is one buffered server-sent event.
 */
type sseEvent struct {
	ID   int
	Data []byte
}

/*
 * mcpSession tracks one MCP session: its negotiated protocol version,
 * a bounded event ring for Last-Event-ID replay and live GET subscribers.
 */
type mcpSession struct {
	id              string
	protocolVersion string

	mu          sync.Mutex
	nextEventID int
	ring        []sseEvent
	subscribers map[chan sseEvent]struct{}
	nextReqID   int
	pending     map[string]chan mcpReply
}

// mcpReply carries one client answer to a server-initiated request.
type mcpReply struct {
	result any
	err    error
}

func newMcpSession(protocolVersion string) *mcpSession {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		log.Panic(err.Error())
	}
	return &mcpSession{
		id:              hex.EncodeToString(buf),
		protocolVersion: protocolVersion,
		subscribers:     make(map[chan sseEvent]struct{}),
		pending:         make(map[string]chan mcpReply),
	}
}

// nextRequestID allocates a server-side JSON-RPC request id.
func (sess *mcpSession) nextRequestID() string {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.nextReqID++
	return fmt.Sprintf("srv-%d", sess.nextReqID)
}

// beginRequest registers a pending server-to-client request.
func (sess *mcpSession) beginRequest(id string) chan mcpReply {
	ch := make(chan mcpReply, 1)
	sess.mu.Lock()
	sess.pending[id] = ch
	sess.mu.Unlock()
	return ch
}

// endRequest removes a pending server-to-client request.
func (sess *mcpSession) endRequest(id string) {
	sess.mu.Lock()
	delete(sess.pending, id)
	sess.mu.Unlock()
}

// deliverResponse routes a client JSON-RPC response to its pending request.
func (sess *mcpSession) deliverResponse(id string, result any, errObj any) bool {
	sess.mu.Lock()
	ch, ok := sess.pending[id]
	sess.mu.Unlock()
	if !ok {
		return false
	}
	reply := mcpReply{result: result}
	if errObj != nil {
		if em, ok := errObj.(map[string]any); ok {
			msg, _ := em["message"].(string)
			reply.err = errors.New(msg)
		} else {
			reply.err = errors.New("mcp request failed")
		}
	}
	ch <- reply
	return true
}

func (sess *mcpSession) nextID() int {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.nextEventID++
	return sess.nextEventID
}

// record appends the event to the replay ring and fans it out to subscribers.
func (sess *mcpSession) record(ev sseEvent) {
	sess.mu.Lock()
	sess.ring = append(sess.ring, ev)
	if len(sess.ring) > mcpEventRingSize {
		sess.ring = sess.ring[len(sess.ring)-mcpEventRingSize:]
	}
	subs := make([]chan sseEvent, 0, len(sess.subscribers))
	for ch := range sess.subscribers {
		subs = append(subs, ch)
	}
	sess.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// replayAfter returns buffered events with an id greater than lastID.
func (sess *mcpSession) replayAfter(lastID int) []sseEvent {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	var out []sseEvent
	for _, ev := range sess.ring {
		if ev.ID > lastID {
			out = append(out, ev)
		}
	}
	return out
}

func (sess *mcpSession) subscribe() (chan sseEvent, func()) {
	ch := make(chan sseEvent, mcpEventRingSize)
	sess.mu.Lock()
	sess.subscribers[ch] = struct{}{}
	sess.mu.Unlock()
	return ch, func() {
		sess.mu.Lock()
		delete(sess.subscribers, ch)
		sess.mu.Unlock()
	}
}

/*
 * StreamableHttpServer implements the MCP streamable HTTP transport on a
 * single /mcp endpoint: POST carries JSON-RPC traffic (unary results as
 * JSON, streaming results as SSE), GET opens a server-to-client SSE channel,
 * DELETE ends the session. Registered services are additionally exposed
 * through the MCP tools layer (tools/list, tools/call).
 */
type StreamableHttpServer struct {
	Hostname  string
	Port      int
	Server    common.Server
	Options   StreamableHttpOptions
	Event     chan int
	Discovery discovery.Driver
	Sessions  sync.Map

	mcpLock      sync.Mutex
	mcpTools     map[string]McpTool
	mcpToolOrder []string
	mcpResources []McpResource
	mcpTemplates []McpResourceTemplate
	mcpPrompts   map[string]McpPrompt
	mcpPromptOrd []string
}

/*
 * McpTool is a manually registered MCP tool. Its handler receives an
 * McpContext that can emit progress/log notifications and send
 * server-to-client requests (sampling/createMessage, elicitation/create).
 * The handler result is wrapped as a CallToolResult; returning a map that
 * already contains a "content" key passes it through unchanged.
 */
type McpTool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     func(ctx *McpContext, arguments map[string]any) (any, error)
}

/*
 * McpResource is a static MCP resource served by resources/list and
 * resources/read. Exactly one of Text or Blob should carry the payload.
 */
type McpResource struct {
	URI         string
	Name        string
	Title       string
	Description string
	MimeType    string
	Text        string
	Blob        []byte
}

/*
 * McpResourceTemplate serves URIs matching a {param} template pattern.
 * Read receives the concrete requested URI.
 */
type McpResourceTemplate struct {
	URITemplate string
	Name        string
	Title       string
	Description string
	MimeType    string
	Read        func(uri string) (text string, blob []byte, ok bool)
}

/*
 * McpPrompt is a manually registered MCP prompt served by prompts/list and
 * prompts/get. Render returns the GetPromptResult map.
 */
type McpPrompt struct {
	Name        string
	Title       string
	Description string
	Arguments   []map[string]any
	Render      func(arguments map[string]any) (map[string]any, error)
}

/*
 * RegisterMcpTool registers an MCP tool with an exact name and schema,
 * bypassing the Service_Method reflection publishing.
 */
func (s *StreamableHttpServer) RegisterMcpTool(tool McpTool) {
	s.mcpLock.Lock()
	defer s.mcpLock.Unlock()
	if s.mcpTools == nil {
		s.mcpTools = make(map[string]McpTool)
	}
	if _, exists := s.mcpTools[tool.Name]; !exists {
		s.mcpToolOrder = append(s.mcpToolOrder, tool.Name)
	}
	s.mcpTools[tool.Name] = tool
}

/*
 * RegisterMcpResource publishes a static resource through resources/list.
 */
func (s *StreamableHttpServer) RegisterMcpResource(r McpResource) {
	s.mcpLock.Lock()
	defer s.mcpLock.Unlock()
	s.mcpResources = append(s.mcpResources, r)
}

/*
 * RegisterMcpResourceTemplate publishes a resource template through
 * resources/templates/list and serves matching URIs.
 */
func (s *StreamableHttpServer) RegisterMcpResourceTemplate(t McpResourceTemplate) {
	s.mcpLock.Lock()
	defer s.mcpLock.Unlock()
	s.mcpTemplates = append(s.mcpTemplates, t)
}

/*
 * RegisterMcpPrompt publishes a prompt through prompts/list and prompts/get.
 */
func (s *StreamableHttpServer) RegisterMcpPrompt(p McpPrompt) {
	s.mcpLock.Lock()
	defer s.mcpLock.Unlock()
	if s.mcpPrompts == nil {
		s.mcpPrompts = make(map[string]McpPrompt)
	}
	if _, exists := s.mcpPrompts[p.Name]; !exists {
		s.mcpPromptOrd = append(s.mcpPromptOrd, p.Name)
	}
	s.mcpPrompts[p.Name] = p
}

/*
 * NewServer creates a new streamable HTTP server
 * @return Server - The new streamable HTTP server
 */
func (p *StreamableHttp) NewServer() Server {
	return &StreamableHttpServer{
		Hostname: "",
		Port:     p.Port,
		Server: common.Server{
			Sm:          sync.Map{},
			Hooks:       common.Hooks{},
			RateLimiter: nil,
		},
		Options:    StreamableHttpOptions{},
		Event:      make(chan int, 1),
		Discovery:  nil,
		mcpTools:   make(map[string]McpTool),
		mcpPrompts: make(map[string]McpPrompt),
	}
}

/*
 * Start starts the streamable HTTP server
 */
func (s *StreamableHttpServer) Start() {
	if s.Discovery != nil {
		register := func(key, value any) bool {
			go s.DiscoveryRegister(key, value)
			return true
		}
		s.Server.Sm.Range(register)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(McpEndpoint, s.handleMcp)
	ln := common.BindListener("0.0.0.0", &s.Port)
	log.Printf("Listening http://0.0.0.0:%d%s", s.Port, McpEndpoint)
	common.NotifyStarted(s.Port, s.Event)
	if err := http.Serve(ln, mux); err != nil {
		log.Panic(err.Error())
	}
}

/*
 * DiscoveryRegister registers a service to the discovery service
 * @param key - The service key
 * @param value - The service value
 * @return bool - True if registration succeeded
 */
func (s *StreamableHttpServer) DiscoveryRegister(key, value any) bool {
	for {
		if err := s.Discovery.Register(key.(string), "http", s.Hostname, s.Port); err == nil {
			return true
		}
		time.Sleep(REGISTRY_RETRY_INTERVAL * time.Millisecond)
	}
}

/*
 * Register registers a service
 * @param m - The service to register
 */
func (s *StreamableHttpServer) Register(m any) {
	err := s.Server.Register(m)
	if err != nil {
		log.Panic(err.Error())
	}
}

/*
 * SetOptions sets the streamable HTTP server options
 * @param options - The streamable HTTP server options
 */
func (s *StreamableHttpServer) SetOptions(options any) {
	s.Options = options.(StreamableHttpOptions)
}

/*
 * SetDiscovery sets the service discovery driver
 * @param d - The service discovery driver
 * @param hostname - The hostname to register
 */
func (s *StreamableHttpServer) SetDiscovery(d discovery.Driver, hostname string) {
	s.Discovery = d
	s.Hostname = hostname
	var err error
	if s.Hostname == "" {
		s.Hostname, err = GetHostname()
		if err != nil {
			common.Debug(err.Error())
		}
	}
}

/*
 * SetRateLimit sets the rate limiter
 * @param r - The rate limit
 * @param b - The burst limit
 */
func (s *StreamableHttpServer) SetRateLimit(r rate.Limit, b int) {
	s.Server.RateLimiter = rate.NewLimiter(r, b)
}

/*
 * SetBeforeFunc sets the before function
 * @param beforeFunc - The before function
 */
func (s *StreamableHttpServer) SetBeforeFunc(beforeFunc func(id any, method string, params any) error) {
	s.Server.Hooks.BeforeFunc = beforeFunc
}

/*
 * SetAfterFunc sets the after function
 * @param afterFunc - The after function
 */
func (s *StreamableHttpServer) SetAfterFunc(afterFunc func(id any, method string, result any) error) {
	s.Server.Hooks.AfterFunc = afterFunc
}

/*
 * GetEvent returns the event channel
 * @return <-chan int - The event channel
 */
func (s *StreamableHttpServer) GetEvent() <-chan int {
	return s.Event
}

/*
 * checkOrigin rejects browsers from origins outside the allow list.
 * Requests without an Origin header (non-browser clients) are always allowed.
 */
func (s *StreamableHttpServer) checkOrigin(r *http.Request) bool {
	if len(s.Options.AllowedOrigins) == 0 {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, allowed := range s.Options.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

/*
 * checkHost rejects requests whose Host header is outside the allow list.
 * Disabled when AllowedHosts is empty.
 */
func (s *StreamableHttpServer) checkHost(r *http.Request) bool {
	if len(s.Options.AllowedHosts) == 0 {
		return true
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	for _, allowed := range s.Options.AllowedHosts {
		a := strings.ToLower(strings.TrimSpace(allowed))
		if h, _, err := net.SplitHostPort(a); err == nil {
			a = h
		}
		if host == strings.Trim(a, "[]") {
			return true
		}
	}
	return false
}

/*
 * handleMcp routes one request on the /mcp endpoint.
 */
func (s *StreamableHttpServer) handleMcp(w http.ResponseWriter, r *http.Request) {
	if !s.checkHost(r) {
		http.Error(w, "forbidden host", http.StatusMisdirectedRequest)
		return
	}
	if !s.checkOrigin(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		s.handleGet(w, r)
	case http.MethodDelete:
		s.handleDelete(w, r)
	default:
		w.Header().Set("Allow", "POST, GET, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

/*
 * resolveSession looks up the session named by the Mcp-Session-Id header.
 * A request without the header is handled statelessly.
 */
func (s *StreamableHttpServer) resolveSession(w http.ResponseWriter, r *http.Request) (*mcpSession, bool) {
	sid := r.Header.Get(McpSessionHeader)
	if sid == "" {
		return nil, true
	}
	v, ok := s.Sessions.Load(sid)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(common.JsonErrorEnvelope(nil, common.InvalidRequest))
		return nil, false
	}
	return v.(*mcpSession), true
}

/*
 * handlePost processes a JSON-RPC request or batch.
 */
func (s *StreamableHttpServer) handlePost(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	parsed, err := common.ParseRequestBody(data)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(common.JsonErrorEnvelope(nil, common.ParseError))
		return
	}
	switch req := parsed.(type) {
	case map[string]any:
		sess, ok := s.resolveSession(w, r)
		if !ok {
			return
		}
		s.handleSingle(w, r, sess, req)
	case []any:
		sess, ok := s.resolveSession(w, r)
		if !ok {
			return
		}
		s.handleBatch(w, sess, req)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(common.JsonErrorEnvelope(nil, common.InvalidRequest))
	}
}

/*
 * handleSingle dispatches one request object, MCP or native.
 */
func (s *StreamableHttpServer) handleSingle(w http.ResponseWriter, r *http.Request, sess *mcpSession, req map[string]any) {
	method, _ := req["method"].(string)
	if method == "" {
		// A JSON-RPC response answering a server-initiated request.
		if sess != nil {
			sess.deliverResponse(fmt.Sprintf("%v", req["id"]), req["result"], req["error"])
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	switch {
	case method == "initialize":
		s.handleInitialize(w, r, req)
	case method == "ping":
		s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{}))
	case method == "tools/list":
		s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{"tools": s.listTools()}))
	case method == "tools/call":
		s.handleToolsCall(w, r, sess, req)
	case method == "resources/list":
		s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{"resources": s.listResources()}))
	case method == "resources/templates/list":
		s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{"resourceTemplates": s.listResourceTemplates()}))
	case method == "resources/read":
		s.handleResourceRead(w, sess, req)
	case method == "resources/subscribe", method == "resources/unsubscribe":
		s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{}))
	case method == "prompts/list":
		s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{"prompts": s.listPrompts()}))
	case method == "prompts/get":
		s.handlePromptGet(w, sess, req)
	case method == "completion/complete":
		s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{
			"completion": map[string]any{"values": []string{}, "total": 0, "hasMore": false},
		}))
	case method == "logging/setLevel":
		s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{}))
	case strings.HasPrefix(method, "notifications/"):
		w.WriteHeader(http.StatusAccepted)
	default:
		if _, hasID := req["id"]; !hasID {
			// Native JSON-RPC notification: execute without a response body.
			s.Server.HandleRequest(req, nil)
			w.WriteHeader(http.StatusAccepted)
			return
		}
		s.dispatchNative(w, sess, req)
	}
}

/*
 * handleInitialize negotiates the protocol version and opens a session.
 */
func (s *StreamableHttpServer) handleInitialize(w http.ResponseWriter, r *http.Request, req map[string]any) {
	negotiated := mcpLatestVersion
	if params, ok := req["params"].(map[string]any); ok {
		if pv, ok := params["protocolVersion"].(string); ok {
			for _, v := range mcpSupportedVersions {
				if v == pv {
					negotiated = pv
				}
			}
		}
	}
	sess := newMcpSession(negotiated)
	s.Sessions.Store(sess.id, sess)
	w.Header().Set(McpSessionHeader, sess.id)
	w.Header().Set(McpProtocolHeader, sess.protocolVersion)
	result := map[string]any{
		"protocolVersion": sess.protocolVersion,
		"capabilities": map[string]any{
			"tools":       map[string]any{},
			"resources":   map[string]any{"subscribe": true},
			"prompts":     map[string]any{},
			"logging":     map[string]any{},
			"completions": map[string]any{},
		},
		"serverInfo": map[string]any{"name": mcpServerInfoName, "version": mcpServerInfoVersion},
	}
	s.respondJSON(w, nil, common.S(req["id"], common.JsonRpc, result))
}

/*
 * handleGet opens the server-to-client SSE channel and replays buffered
 * events when the client passes Last-Event-ID.
 */
func (s *StreamableHttpServer) handleGet(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("sessionId")
	if sid == "" {
		sid = r.Header.Get(McpSessionHeader)
	}
	if sid == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	v, ok := s.Sessions.Load(sid)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	sess := v.(*mcpSession)
	cw := &sseWriter{w: w, sess: sess}
	cw.start()
	lastID := 0
	if raw := r.Header.Get(McpLastEventHeader); raw != "" {
		lastID, _ = strconv.Atoi(raw)
	}
	for _, ev := range sess.replayAfter(lastID) {
		cw.writeRaw(ev)
	}

	ch, unsub := sess.subscribe()
	defer unsub()
	for {
		select {
		case ev := <-ch:
			cw.writeRaw(ev)
		case <-r.Context().Done():
			return
		}
	}
}

/*
 * handleDelete ends a session.
 */
func (s *StreamableHttpServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	sid := r.Header.Get(McpSessionHeader)
	if sid == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.Sessions.LoadAndDelete(sid); !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

/*
 * handleBatch executes every request of a batch with stream results
 * collected into arrays, answering with one JSON array.
 */
func (s *StreamableHttpServer) handleBatch(w http.ResponseWriter, sess *mcpSession, list []any) {
	if len(list) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(common.JsonErrorEnvelope(nil, common.InvalidRequest))
		return
	}
	responses := make([]any, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			responses = append(responses, common.E(nil, common.JsonRpc, common.InvalidRequest))
			continue
		}
		resp, _ := s.Server.HandleRequest(m, nil)
		if resp != nil {
			responses = append(responses, resp)
		}
	}
	s.respondJSON(w, sess, responses)
}

/*
 * dispatchNative runs a native JSON-RPC request, answering with JSON for
 * unary results and with SSE when the method result is a stream.
 */
func (s *StreamableHttpServer) dispatchNative(w http.ResponseWriter, sess *mcpSession, req map[string]any) {
	id := req["id"]
	cw := &sseWriter{w: w, sess: sess}
	index := 0
	stream := &common.StreamHandler{
		Begin: cw.start,
		OnValue: func(v any) error {
			data, err := json.Marshal(map[string]any{
				"jsonrpc": common.JsonRpc,
				"method":  "notifications/stream",
				"params":  map[string]any{"id": id, "index": index, "value": v},
			})
			if err != nil {
				return err
			}
			index++
			return cw.event(data)
		},
	}
	resp, streamed := s.Server.HandleRequest(req, stream)
	if resp == nil {
		return
	}
	if streamed {
		cw.writeJSON(resp)
		return
	}
	s.respondJSON(w, sess, resp)
}

/*
 * respondJSON writes one or more response objects as application/json.
 */
func (s *StreamableHttpServer) respondJSON(w http.ResponseWriter, sess *mcpSession, resp any) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	if sess != nil {
		h.Set(McpProtocolHeader, sess.protocolVersion)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("write response failed: %v", err)
	}
}

/*
 * sseWriter writes SSE events, optionally registering them in the session
 * ring for Last-Event-ID replay and live GET fan-out.
 */
type sseWriter struct {
	w       http.ResponseWriter
	sess    *mcpSession
	started bool
}

func (e *sseWriter) start() {
	if e.started {
		return
	}
	h := e.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	if e.sess != nil {
		h.Set(McpProtocolHeader, e.sess.protocolVersion)
	}
	e.w.WriteHeader(http.StatusOK)
	e.started = true
	// SEP-1699: advertise the reconnect interval and emit an id-bearing
	// priming event so clients can resume with Last-Event-ID.
	fmt.Fprint(e.w, "retry: 2000\n\n")
	if e.sess != nil {
		fmt.Fprintf(e.w, "id: %d\ndata: \n\n", e.sess.nextID())
	}
	e.flush()
}

func (e *sseWriter) flush() {
	if f, ok := e.w.(http.Flusher); ok {
		f.Flush()
	}
}

// event assigns a session event id (when sessionized), buffers the event for
// replay and writes it to the stream.
func (e *sseWriter) event(data []byte) error {
	if !e.started {
		e.start()
	}
	ev := sseEvent{Data: data}
	if e.sess != nil {
		ev.ID = e.sess.nextID()
		e.sess.record(ev)
	}
	e.writeRaw(ev)
	return nil
}

func (e *sseWriter) writeRaw(ev sseEvent) {
	if ev.ID > 0 {
		fmt.Fprintf(e.w, "id: %d\n", ev.ID)
	}
	fmt.Fprintf(e.w, "event: message\ndata: %s\n\n", ev.Data)
	e.flush()
}

func (e *sseWriter) writeJSON(resp any) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	if err := e.event(data); err != nil {
		log.Printf("write SSE event failed: %v", err)
	}
}

/*
 * listTools publishes every registered method as an MCP tool named
 * Service_Method. Go reflection cannot read method parameter names, so
 * value parameters are named arg0..argN while struct parameters are
 * expanded into their JSON-tagged fields.
 */
func (s *StreamableHttpServer) listTools() []map[string]any {
	tools := []map[string]any{}
	s.Server.Sm.Range(func(_, v any) bool {
		svc := v.(*common.Service)
		names := make([]string, 0, len(svc.Mm))
		for name := range svc.Mm {
			names = append(names, name)
		}
		// Stable order for deterministic tools/list output.
		sort.Strings(names)
		for _, name := range names {
			m := svc.Mm[name]
			tools = append(tools, map[string]any{
				"name":        svc.Name + "_" + name,
				"description": fmt.Sprintf("jsonrpc4go service method %s/%s", svc.Name, name),
				"inputSchema": inputSchema(m.ParamsTypes),
			})
		}
		return true
	})
	s.mcpLock.Lock()
	for _, name := range s.mcpToolOrder {
		t := s.mcpTools[name]
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}}
		}
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": schema,
		})
	}
	s.mcpLock.Unlock()
	return tools
}

// argBinding maps one method parameter to its tool argument names.
type argBinding struct {
	name   string   // "argK" for value parameters
	fields []string // JSON field names for struct parameters
}

func bindParams(types []reflect.Type) []argBinding {
	binds := make([]argBinding, 0, len(types))
	for k, t := range types {
		if t.Kind() == reflect.Struct {
			var fields []string
			for f := range t.NumField() {
				if t.Field(f).IsExported() && !jsonSkipped(t.Field(f)) {
					fields = append(fields, jsonFieldName(t.Field(f)))
				}
			}
			binds = append(binds, argBinding{fields: fields})
		} else {
			binds = append(binds, argBinding{name: "arg" + strconv.Itoa(k)})
		}
	}
	return binds
}

// jsonSkipped reports whether a struct field is never decoded from JSON
// because it carries the `json:"-"` tag.
func jsonSkipped(f reflect.StructField) bool {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name == "-"
}

func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag != "" {
		if name, _, _ := strings.Cut(tag, ","); name != "" && name != "-" {
			return name
		}
	}
	return strings.ToLower(f.Name)
}

func inputSchema(types []reflect.Type) map[string]any {
	properties := map[string]any{}
	required := []string{}
	for k, t := range types {
		if t.Kind() == reflect.Struct {
			for f := range t.NumField() {
				if !t.Field(f).IsExported() || jsonSkipped(t.Field(f)) {
					continue
				}
				name := jsonFieldName(t.Field(f))
				properties[name] = map[string]any{"type": jsonTypeName(t.Field(f).Type)}
				required = append(required, name)
			}
			continue
		}
		name := "arg" + strconv.Itoa(k)
		properties[name] = map[string]any{"type": jsonTypeName(t)}
		required = append(required, name)
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func jsonTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return "object"
	}
}

/*
 * handleToolsCall executes one MCP tool call. Streaming tool results are
 * additionally announced as notifications/progress SSE events when the
 * client supplied _meta.progressToken.
 */
func (s *StreamableHttpServer) handleToolsCall(w http.ResponseWriter, r *http.Request, sess *mcpSession, req map[string]any) {
	id := req["id"]
	params, _ := req["params"].(map[string]any)
	name, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]any)
	if arguments == nil {
		arguments = map[string]any{}
	}
	meta, _ := params["_meta"].(map[string]any)
	token, hasToken := meta["progressToken"]

	if tool, ok := s.mcpTool(name); ok {
		s.callMcpTool(w, sess, id, tool, token, arguments)
		return
	}

	sp := strings.SplitN(name, "_", 2)
	if len(sp) != 2 {
		s.respondJSON(w, sess, common.E(id, common.JsonRpc, common.InvalidParams))
		return
	}
	_, m := s.lookupTool(sp[0], sp[1])
	if m == nil {
		s.respondJSON(w, sess, common.E(id, common.JsonRpc, common.MethodNotFound))
		return
	}

	positional := bindArguments(bindParams(m.ParamsTypes), m.ParamsTypes, arguments)
	req2 := map[string]any{
		"jsonrpc": common.JsonRpc,
		"id":      id,
		"method":  sp[0] + "/" + sp[1],
		"params":  positional,
	}

	var (
		cw     = &sseWriter{w: w, sess: sess}
		stream *common.StreamHandler
	)
	if hasToken {
		progress := 0
		stream = &common.StreamHandler{
			Begin: cw.start,
			OnValue: func(v any) error {
				progress++
				data, err := json.Marshal(map[string]any{
					"jsonrpc": common.JsonRpc,
					"method":  "notifications/progress",
					"params":  map[string]any{"progressToken": token, "progress": progress},
				})
				if err != nil {
					return err
				}
				return cw.event(data)
			},
		}
	}
	resp, streamed := s.Server.HandleRequest(req2, stream)
	if resp == nil {
		return
	}
	toolResp := callToolResponse(id, resp)
	if streamed {
		cw.writeJSON(toolResp)
		return
	}
	s.respondJSON(w, sess, toolResp)
}

/*
 * mcpTool returns a manually registered tool by exact name.
 */
func (s *StreamableHttpServer) mcpTool(name string) (McpTool, bool) {
	s.mcpLock.Lock()
	defer s.mcpLock.Unlock()
	t, ok := s.mcpTools[name]
	return t, ok
}

/*
 * callMcpTool runs a custom tool handler. The handler may emit SSE events
 * (progress, logs, server-to-client requests) through the McpContext; the
 * final CallToolResult is delivered as JSON when nothing was streamed, or as
 * the last SSE event otherwise.
 */
func (s *StreamableHttpServer) callMcpTool(w http.ResponseWriter, sess *mcpSession, id any, tool McpTool, token any, arguments map[string]any) {
	cw := &sseWriter{w: w, sess: sess}
	ctx := &McpContext{sess: sess, cw: cw, id: id, token: token}
	out, err := tool.Handler(ctx, arguments)
	if ctx.ended {
		// The handler closed the stream; the response arrives via SendResult.
		return
	}
	resp := mcpToolResponse(id, out, err)
	if cw.started {
		cw.writeJSON(resp)
		return
	}
	s.respondJSON(w, sess, resp)
}

/*
 * mcpToolResponse converts a custom handler result into a JSON-RPC response
 * carrying a CallToolResult. Maps that already contain a "content" key pass
 * through unchanged so handlers can build full CallToolResults.
 */
func mcpToolResponse(id any, out any, err error) any {
	if err != nil {
		return common.S(id, common.JsonRpc, map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		})
	}
	if m, ok := out.(map[string]any); ok {
		if _, has := m["content"]; has {
			return common.S(id, common.JsonRpc, m)
		}
	}
	text, mErr := json.Marshal(out)
	if mErr != nil {
		text = []byte(fmt.Sprintf("%v", out))
	}
	return common.S(id, common.JsonRpc, map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
		"isError": false,
	})
}

/*
 * McpContext gives custom tool handlers access to the live MCP stream:
 * notifications, server-to-client requests and stream control.
 */
type McpContext struct {
	sess  *mcpSession
	cw    *sseWriter
	id    any
	token any
	ended bool
}

// ProtocolVersion returns the negotiated MCP protocol version.
func (c *McpContext) ProtocolVersion() string {
	if c.sess == nil {
		return mcpLatestVersion
	}
	return c.sess.protocolVersion
}

func (c *McpContext) notify(method string, params map[string]any) error {
	data, err := json.Marshal(map[string]any{
		"jsonrpc": common.JsonRpc,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	return c.cw.event(data)
}

/*
 * Log emits a notifications/message event on the current stream.
 */
func (c *McpContext) Log(level string, logger string, data any) error {
	params := map[string]any{"level": level, "data": data}
	if logger != "" {
		params["logger"] = logger
	}
	return c.notify("notifications/message", params)
}

/*
 * Progress emits a notifications/progress event when the client supplied a
 * progressToken. Zero total/message are omitted.
 */
func (c *McpContext) Progress(progress float64, total float64, message string) error {
	if c.token == nil {
		return nil
	}
	params := map[string]any{"progressToken": c.token, "progress": progress}
	if total > 0 {
		params["total"] = total
	}
	if message != "" {
		params["message"] = message
	}
	return c.notify("notifications/progress", params)
}

/*
 * Request sends a server-to-client JSON-RPC request (e.g.
 * sampling/createMessage or elicitation/create) over the live stream and
 * waits for the client's response.
 */
func (c *McpContext) Request(method string, params map[string]any, timeout time.Duration) (any, error) {
	if c.sess == nil {
		return nil, errors.New("mcp: server-to-client requests require a session")
	}
	rid := c.sess.nextRequestID()
	ch := c.sess.beginRequest(rid)
	defer c.sess.endRequest(rid)
	data, err := json.Marshal(map[string]any{
		"jsonrpc": common.JsonRpc,
		"id":      rid,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	if err := c.cw.event(data); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case reply := <-ch:
		return reply.result, reply.err
	case <-time.After(timeout):
		return nil, errors.New("mcp: timed out waiting for client response")
	}
}

/*
 * EndStream closes the response stream without a final result. The handler
 * (typically in a goroutine) must later call SendResult so reconnecting
 * clients receive it through Last-Event-ID replay.
 */
func (c *McpContext) EndStream() {
	c.ended = true
}

/*
 * SendResult records the final response for a stream ended with EndStream,
 * making it available to clients reconnecting with Last-Event-ID.
 */
func (c *McpContext) SendResult(result any) {
	if c.sess == nil {
		return
	}
	data, err := json.Marshal(common.S(c.id, common.JsonRpc, result))
	if err != nil {
		return
	}
	c.sess.record(sseEvent{ID: c.sess.nextID(), Data: data})
}

/*
 * listResources returns the registered static resources.
 */
func (s *StreamableHttpServer) listResources() []map[string]any {
	s.mcpLock.Lock()
	defer s.mcpLock.Unlock()
	out := make([]map[string]any, 0, len(s.mcpResources))
	for _, r := range s.mcpResources {
		item := map[string]any{"uri": r.URI, "name": r.Name}
		if r.Title != "" {
			item["title"] = r.Title
		}
		if r.Description != "" {
			item["description"] = r.Description
		}
		if r.MimeType != "" {
			item["mimeType"] = r.MimeType
		}
		out = append(out, item)
	}
	return out
}

/*
 * listResourceTemplates returns the registered resource templates.
 */
func (s *StreamableHttpServer) listResourceTemplates() []map[string]any {
	s.mcpLock.Lock()
	defer s.mcpLock.Unlock()
	out := make([]map[string]any, 0, len(s.mcpTemplates))
	for _, t := range s.mcpTemplates {
		item := map[string]any{"uriTemplate": t.URITemplate, "name": t.Name}
		if t.Title != "" {
			item["title"] = t.Title
		}
		if t.Description != "" {
			item["description"] = t.Description
		}
		if t.MimeType != "" {
			item["mimeType"] = t.MimeType
		}
		out = append(out, item)
	}
	return out
}

/*
 * matchResourceTemplate reports whether uri matches a template such as
 * "test://template/{id}/data": every literal segment must match and each
 * {param} segment accepts any non-empty segment.
 */
func matchResourceTemplate(tmpl, uri string) bool {
	ts := strings.Split(tmpl, "/")
	us := strings.Split(uri, "/")
	if len(ts) != len(us) {
		return false
	}
	for k, seg := range ts {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			if us[k] == "" {
				return false
			}
			continue
		}
		if seg != us[k] {
			return false
		}
	}
	return true
}

/*
 * handleResourceRead resolves resources/read against registered resources
 * and templates.
 */
func (s *StreamableHttpServer) handleResourceRead(w http.ResponseWriter, sess *mcpSession, req map[string]any) {
	params, _ := req["params"].(map[string]any)
	uri, _ := params["uri"].(string)
	s.mcpLock.Lock()
	var (
		text  string
		blob  []byte
		mime  string
		found bool
	)
	for _, r := range s.mcpResources {
		if r.URI == uri {
			text, blob, mime, found = r.Text, r.Blob, r.MimeType, true
			break
		}
	}
	var tmpl *McpResourceTemplate
	if !found {
		for k := range s.mcpTemplates {
			if matchResourceTemplate(s.mcpTemplates[k].URITemplate, uri) {
				tmpl = &s.mcpTemplates[k]
				break
			}
		}
	}
	s.mcpLock.Unlock()
	if !found && tmpl == nil {
		s.respondJSON(w, sess, common.E(req["id"], common.JsonRpc, common.InvalidParams))
		return
	}
	if tmpl != nil {
		if tmpl.Read == nil {
			s.respondJSON(w, sess, common.E(req["id"], common.JsonRpc, common.InvalidParams))
			return
		}
		var ok bool
		text, blob, ok = tmpl.Read(uri)
		mime = tmpl.MimeType
		if !ok {
			s.respondJSON(w, sess, common.E(req["id"], common.JsonRpc, common.InvalidParams))
			return
		}
	}
	if mime == "" {
		mime = resourceMimeType(text, blob)
	}
	item := map[string]any{"uri": uri, "mimeType": mime}
	if len(blob) > 0 {
		item["blob"] = base64.StdEncoding.EncodeToString(blob)
	} else {
		item["text"] = text
	}
	s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, map[string]any{
		"contents": []map[string]any{item},
	}))
}

func resourceMimeType(text string, blob []byte) string {
	if len(blob) > 0 {
		return "application/octet-stream"
	}
	return "text/plain"
}

/*
 * listPrompts returns the registered prompts.
 */
func (s *StreamableHttpServer) listPrompts() []map[string]any {
	s.mcpLock.Lock()
	defer s.mcpLock.Unlock()
	out := make([]map[string]any, 0, len(s.mcpPromptOrd))
	for _, name := range s.mcpPromptOrd {
		p := s.mcpPrompts[name]
		item := map[string]any{"name": p.Name}
		if p.Title != "" {
			item["title"] = p.Title
		}
		if p.Description != "" {
			item["description"] = p.Description
		}
		if len(p.Arguments) > 0 {
			item["arguments"] = p.Arguments
		}
		out = append(out, item)
	}
	return out
}

/*
 * handlePromptGet renders a registered prompt.
 */
func (s *StreamableHttpServer) handlePromptGet(w http.ResponseWriter, sess *mcpSession, req map[string]any) {
	params, _ := req["params"].(map[string]any)
	name, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]any)
	s.mcpLock.Lock()
	p, ok := s.mcpPrompts[name]
	s.mcpLock.Unlock()
	if !ok || p.Render == nil {
		s.respondJSON(w, sess, common.E(req["id"], common.JsonRpc, common.InvalidParams))
		return
	}
	result, err := p.Render(arguments)
	if err != nil {
		s.respondJSON(w, sess, common.CE(req["id"], common.JsonRpc, err.Error()))
		return
	}
	s.respondJSON(w, sess, common.S(req["id"], common.JsonRpc, result))
}

/*
 * lookupTool resolves a tool name back to a registered service method.
 */
func (s *StreamableHttpServer) lookupTool(sName, mName string) (*common.Service, *common.Method) {
	v, ok := s.Server.Sm.Load(sName)
	if !ok {
		v, ok = s.Server.Sm.Load(common.LineToHump(sName))
	}
	if !ok {
		return nil, nil
	}
	svc := v.(*common.Service)
	m, ok := svc.Mm[mName]
	if !ok {
		return nil, nil
	}
	return svc, m
}

/*
 * bindArguments converts a tool arguments object into the positional
 * parameter list understood by the RPC dispatcher.
 */
func bindArguments(binds []argBinding, types []reflect.Type, arguments map[string]any) []any {
	positional := make([]any, 0, len(binds))
	for _, b := range binds {
		if len(b.fields) == 0 {
			positional = append(positional, arguments[b.name])
			continue
		}
		obj := map[string]any{}
		for _, f := range b.fields {
			obj[f] = arguments[f]
		}
		positional = append(positional, obj)
	}
	return positional
}

/*
 * callToolResponse converts a JSON-RPC response into an MCP CallToolResult
 * envelope. Internal and custom errors become tool errors (isError);
 * protocol errors stay JSON-RPC errors.
 */
func callToolResponse(id any, resp any) any {
	switch res := resp.(type) {
	case common.SuccessResponse:
		text, err := json.Marshal(res.Result)
		if err != nil {
			text = []byte(fmt.Sprintf("%v", res.Result))
		}
		return common.S(id, common.JsonRpc, map[string]any{
			"content": []map[string]any{{"type": "text", "text": string(text)}},
			"isError": false,
		})
	case common.ErrorResponse:
		switch res.Error.Code {
		case common.InternalError, common.CustomError:
			return common.S(id, common.JsonRpc, map[string]any{
				"content": []map[string]any{{"type": "text", "text": res.Error.Message}},
				"isError": true,
			})
		default:
			return resp
		}
	default:
		return resp
	}
}
