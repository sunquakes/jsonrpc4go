package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"github.com/sunquakes/jsonrpc4go/discovery/etcd/etcdserverpb"
	"google.golang.org/grpc"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Etcd struct {
	URL  *url.URL
	Conn *grpc.ClientConn
}

type Service struct {
	UniqueId string
	Name     string
	Addr     string
}

func NewEtcd(rawURL string) (discovery.Driver, error) {
	URL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	// Create a connection to the etcd server
	conn, err := grpc.Dial(URL.Host, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	etcd := &Etcd{URL, conn}
	return etcd, nil
}

func (d *Etcd) Register(name string, protocol string, hostname string, port int) error {
	var addr string
	if protocol == "http" || protocol == "https" {
		addr = fmt.Sprintf("%s://%s:%d", protocol, hostname, port)
	} else {
		addr = fmt.Sprintf("%s:%d", hostname, port)
	}

	// Create a Lease client
	leaseClient := etcdserverpb.NewLeaseClient(d.Conn)

	// Create a KV client
	kvClient := etcdserverpb.NewKVClient(d.Conn)

	// Grant a new lease
	grantResp, err := leaseClient.LeaseGrant(context.Background(), &etcdserverpb.LeaseGrantRequest{TTL: 10})
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
	return nil
}

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
