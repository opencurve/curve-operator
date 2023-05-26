package config

import (
	"regexp"

	"github.com/pkg/errors"
)

/*
 * built-in variables:
 *
 * service:
 *   ${prefix}                     "/curvebs/{etcd,mds,chunkserver}"
 *   ${service_id}                 "c690bde11d1a"
 *   ${service_role}               "mds"
 *   ${service_host}               "10.0.0.1"
 *   ${service_host_sequence}      "1"
 *   ${service_replicas_sequence}  "1"
 *   ${format_replicas_sequence}   "01"
 *   ${service_addr}               "10.0.0.1"
 *   ${service_port}               "6666"
 *   ${service_client_port}        "2379" (etcd)
 *   ${service_dummy_port}         "6667" (snapshotclone/mds)
 *   ${service_proxy_port}         "8080" (snapshotclone)
 *   ${service_external_addr}      "10.0.10.1" (chunkserver/metaserver)
 *   ${service_external_port}      "7800" (metaserver)
 *   ${log_dir}                    "/data/logs"
 *   ${data_dir}                   "/data"
 *   ${random_uuid}                "6fa8f01c411d7655d0354125c36847bb"
 *
 * cluster:
 *   ${cluster_etcd_http_addr}                "etcd1=http://10.0.10.1:2380,etcd2=http://10.0.10.2:2380,etcd3=http://10.0.10.3:2380"
 *   ${cluster_etcd_addr}                     "10.0.10.1:2380,10.0.10.2:2380,10.0.10.3:2380"
 *   ${cluster_mds_addr}                      "10.0.10.1:6666,10.0.10.2:6666,10.0.10.3:6666"
 *   ${cluster_mds_dummy_addr}                "10.0.10.1:6667,10.0.10.2:6667,10.0.10.3:6667"
 *   ${cluster_mds_dummy_port}                "6667,6668,6669"
 *   ${cluster_chunkserver_addr}              "10.0.10.1:6800,10.0.10.2:6800,10.0.10.3:6800"
 *   ${cluster_snapshotclone_addr}            "10.0.10.1:5555,10.0.10.2:5555,10.0.10.3:5555"
 *   ${cluster_snapshotclone_proxy_addr}      "10.0.10.1:8080,10.0.10.2:8080,10.0.10.3:8083"
 *   ${cluster_snapshotclone_dummy_port}      "8081,8082,8083"
 *   ${cluster_snapshotclone_nginx_upstream}  "server 10.0.0.1:5555; server 10.0.0.3:5555; server 10.0.0.3:5555;"
 *   ${cluster_metaserver_addr}               "10.0.10.1:6701,10.0.10.2:6701,10.0.10.3:6701"
 */

const (
	REGEX_VARIABLE = `\${([^${}]+)}` // ${var_name}
)

//nolint:unused
const (
	ROLE_ETCD          = "etcd"
	ROLE_MDS           = "mds"
	ROLE_CHUNKSERVER   = "chunkserver"
	ROLE_SNAPSHOTCLONE = "snapshotclone"
	ROLE_METASERVER    = "metaserver"
)

type ConfigInterface interface {
	GetPrefix() string
	GetServiceId() string
	GetServiceRole() string
	GetServiceHost() string
	GetServiceHostSequence() string
	GetServiceReplicaSequence() string
	GetServiceReplicasSequence() string
	GetServiceAddr() string
	GetServicePort() string
	GetServiceClientPort() string
	GetServiceDummyPort() string
	GetServiceProxyPort() string
	GetServiceExternalAddr() string
	GetServiceExternalPort() string
	GetLogDir() string
	GetDataDir() string
	// cluster
	GetClusterEtcdHttpAddr() string
	GetClusterEtcdAddr() string
	GetClusterMdsAddr() string
	GetClusterMdsDummyAddr() string
	GetClusterMdsDummyPort() string
	GetClusterChunkserverAddr() string
	GetClusterMetaserverAddr() string
	GetClusterSnapshotcloneAddr() string
	GetClusterSnapshotcloneProxyAddr() string
	GetClusterSnapshotcloneDummyPort() string
	GetClusterSnapshotcloneNginxUpstream() string
}

func getValue(name string, dc ConfigInterface) string {
	switch name {
	case "prefix":
		return dc.GetPrefix()
	case "service_id":
		return dc.GetServiceId()
	case "service_role":
		return dc.GetServiceRole()
	case "service_host":
		return dc.GetServiceHost()
	case "service_host_sequence":
		return dc.GetServiceHostSequence()
	case "service_replica_sequence":
		return dc.GetServiceReplicaSequence()
	case "service_replicas_sequence":
		return dc.GetServiceReplicasSequence()
	case "service_addr":
		return dc.GetServiceAddr()
	case "service_port":
		return dc.GetServicePort()
	case "service_client_port": // etcd
		return dc.GetServiceClientPort()
	case "service_dummy_port": // mds, snapshotclone
		return dc.GetServiceDummyPort()
	case "service_proxy_port": // snapshotclone
		return dc.GetServiceProxyPort()
	case "service_external_addr": // chunkserver, metaserver
		return dc.GetServiceExternalAddr()
	case "service_external_port": // metaserver
		return dc.GetServiceExternalPort()
	case "log_dir":
		return dc.GetLogDir()
	case "data_dir":
		return dc.GetDataDir()
	case "cluster_etcd_http_addr":
		return dc.GetClusterEtcdHttpAddr()
	case "cluster_etcd_addr":
		return dc.GetClusterEtcdAddr()
	case "cluster_mds_addr":
		return dc.GetClusterMdsAddr()
	case "cluster_mds_dummy_addr":
		return dc.GetClusterMdsDummyAddr()
	case "cluster_mds_dummy_port":
		return dc.GetClusterMdsDummyPort()
	case "cluster_chunkserver_addr":
		return dc.GetClusterChunkserverAddr()
	case "cluster_metaserver_addr":
		return dc.GetClusterMetaserverAddr()
	case "cluster_snapshotclone_addr":
		return dc.GetClusterSnapshotcloneAddr()
	case "cluster_snapshotclone_proxy_addr":
		return dc.GetClusterSnapshotcloneProxyAddr()
	case "cluster_snapshotclone_dummy_port":
		return dc.GetClusterSnapshotcloneDummyPort()
	case "cluster_snapshotclone_nginx_upstream":
		return dc.GetClusterSnapshotcloneNginxUpstream()
	}

	return ""
}

// ReplaceConfigVars replaces vars in config string
func ReplaceConfigVars(confStr string, c ConfigInterface) (string, error) {
	r, err := regexp.Compile(REGEX_VARIABLE)
	if err != nil {
		return "", err
	}

	matches := r.ReplaceAllStringFunc(confStr, func(keyName string) string {
		return getValue(keyName[2:len(keyName)-1], c)
	})

	if len(matches) == 0 {
		logger.Error("No matches for regexp")
		return "", errors.Wrap(err, "No matches for regexp")
	}

	return matches, nil
}
