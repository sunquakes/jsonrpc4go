package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sunquakes/jsonrpc4go/v2/discovery"
	"github.com/sunquakes/jsonrpc4go/v2/discovery/etcd/etcdserverpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

/**
 * @Description: Lease TTL (seconds)
 */
var TTL int = 10

/**
 * @Description: Heartbeat interval
 */
var INTERVAL time.Duration = 5 * time.Second

/**
 * @Description: HTTP protocol
 */
var PROTOCOL_HTTP string = "http"

/**
 * @Description: HTTPS protocol
 */
var PROTOCOL_HTTPS string = "https"

/**
 * @Description: Etcd client structure, implements discovery.Driver interface
 * @Field URL: Etcd server URL address
 * @Field Conn: gRPC connection
 * @Field Heartbeat: Legacy heartbeat channel kept for compatibility
 * @Field done: Channel closed by Close to stop heartbeat goroutines
 */
type Etcd struct {
	URL       *url.URL
	Conn      *grpc.ClientConn
	Heartbeat chan bool

	mu   sync.Mutex
	done chan struct{}
}

/**
 * @Description: Service structure
 * @Field UniqueId: Unique identifier
 * @Field Name: Service name
 * @Field Addr: Service address
 */
type Service struct {
	UniqueId string
	Name     string
	Addr     string
}

/**
 * @Description: Create Etcd client instance
 * @Param rawURL: Etcd server URL address
 * @Return discovery.Driver: Service discovery driver instance
 * @Return error: Error message
 */
func NewEtcd(rawURL string) (discovery.Driver, error) {
	URL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	// Create a connection to the etcd server
	conn, err := grpc.NewClient(URL.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	heartbeat := make(chan bool)
	etcd := &Etcd{URL: URL, Conn: conn, Heartbeat: heartbeat, done: make(chan struct{})}
	return etcd, nil
}

/**
 * @Description: Register service
 * @Receiver d: Etcd structure pointer
 * @Param name: Service name
 * @Param protocol: Protocol type
 * @Param hostname: Hostname
 * @Param port: Port number
 * @Return error: Error message
 */
func (d *Etcd) Register(name string, protocol string, hostname string, port int) error {
	var addr string
	if protocol == PROTOCOL_HTTP || protocol == PROTOCOL_HTTPS {
		addr = fmt.Sprintf("%s://%s:%d", protocol, hostname, port)
	} else {
		addr = fmt.Sprintf("%s:%d", hostname, port)
	}

	// Create a Lease client
	leaseClient := etcdserverpb.NewLeaseClient(d.Conn)

	// Create a KV client
	kvClient := etcdserverpb.NewKVClient(d.Conn)

	// Grant a new lease
	grantResp, err := leaseClient.LeaseGrant(context.Background(), &etcdserverpb.LeaseGrantRequest{TTL: int64(TTL)})
	if err != nil {
		return err
	}

	leaseID := grantResp.ID
	data, err := json.Marshal(Service{
		strconv.FormatInt(time.Now().Unix(), 10),
		name,
		addr,
	})
	if err != nil {
		return err
	}
	_, err = kvClient.Put(context.Background(), &etcdserverpb.PutRequest{Key: name, Value: data, Lease: leaseID})
	if err != nil {
		return err
	}
	d.SendHeartbeat(func() {
		_, _ = leaseClient.LeaseKeepAlive(context.Background(), &etcdserverpb.LeaseKeepAliveRequest{ID: leaseID})
	})
	return nil
}

/**
 * @Description: Get service address list
 * @Receiver d: Etcd structure pointer
 * @Param name: Service name
 * @Return string: Service address list (comma separated)
 * @Return error: Error message
 */
func (d *Etcd) Get(name string) (string, error) {
	// Create a KV client
	kvClient := etcdserverpb.NewKVClient(d.Conn)
	resp, err := kvClient.Range(context.Background(), &etcdserverpb.RangeRequest{Key: name})
	if err != nil {
		return "", err
	}
	var servers []string
	for _, item := range resp.Kvs {
		service := &Service{}
		err := json.Unmarshal(item.Value, service)
		if err != nil {
			return "", err
		}
		servers = append(servers, service.Addr)
	}
	return strings.Join(servers, ","), nil
}

/**
 * @Description: Send heartbeat
 * @Receiver d: Etcd structure pointer
 * @Param f: Heartbeat callback function
 */
func (d *Etcd) SendHeartbeat(f func()) {
	done := d.stopChan()
	go func() {
		// Beat once immediately, then on every tick until Close is called.
		ticker := time.NewTicker(INTERVAL)
		defer ticker.Stop()
		f()
		for {
			select {
			case <-ticker.C:
				f()
			case <-done:
				return
			}
		}
	}()
}

/**
 * @Description: Lazily create the shutdown channel so drivers built as struct
 * literals (without NewEtcd) still support cancellation
 * @Receiver d: Etcd structure pointer
 * @Return chan struct{}: Channel closed by Close
 */
func (d *Etcd) stopChan() chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.done == nil {
		d.done = make(chan struct{})
	}
	return d.done
}

/**
 * @Description: Stop heartbeat goroutines and close the gRPC connection
 * @Receiver d: Etcd structure pointer
 * @Return error: Error message from closing the connection
 */
func (d *Etcd) Close() error {
	done := d.stopChan()
	d.mu.Lock()
	select {
	case <-done:
	default:
		close(done)
	}
	d.mu.Unlock()
	if d.Conn != nil {
		return d.Conn.Close()
	}
	return nil
}
