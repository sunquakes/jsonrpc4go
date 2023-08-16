package etcd

import (
	"context"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"github.com/sunquakes/jsonrpc4go/discovery/etcd/etcdserverpb"
	"google.golang.org/grpc"
	"net/url"
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
	defer conn.Close()
	etcd := &Etcd{URL, conn}
	return etcd, nil
}

func (d *Etcd) Register(name string, protocol string, hostname string, port int) error {
	// Create a Lease client
	// leaseClient := etcdserverpb.NewLeaseClient(d.Conn)

	// Create a KV client
	kvClient := etcdserverpb.NewKVClient(d.Conn)
	//
	// // Grant a new lease
	// grantResp, err := leaseClient.LeaseGrant(context.Background(), &etcdserverpb.LeaseGrantRequest{TTL: 10})
	// if err != nil {
	// 	return err
	// }
	//
	// leaseID := grantResp.ID
	// data, err := json.Marshal(Service{})
	// if err != nil {
	// 	return err
	// }
	kvClient.Put(context.Background(), &etcdserverpb.PutRequest{Key: name, Value: "", Lease: 1})
	return nil
}

func (d *Etcd) Get(name string) (string, error) {
	return "", nil
}
