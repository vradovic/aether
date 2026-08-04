package core

import (
	"net"

	"github.com/gocql/gocql"
)

// NewScyllaCluster configures a ScyllaDB cluster for the message store. When a
// single loopback host is given the driver is pinned to it, because the peers
// Scylla gossips inside Docker are not routable from the host.
func NewScyllaCluster(hosts []string, keyspace string) *gocql.ClusterConfig {
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum

	if len(hosts) == 1 && (hosts[0] == "127.0.0.1" || hosts[0] == "localhost") {
		cluster.AddressTranslator = gocql.AddressTranslatorFunc(func(ip net.IP, port int) (net.IP, int) {
			return net.ParseIP(hosts[0]), port
		})
	}

	return cluster
}
