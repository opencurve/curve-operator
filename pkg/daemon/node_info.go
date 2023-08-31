package daemon

import "github.com/opencurve/curve-operator/pkg/k8sutil"

type NodeInfo struct {
	NodeName               string
	NodeIP                 string
	HostID                 int
	ReplicasSequence       int
	PeerPort               int // etcd
	ClientPort             int // etcd
	SnapshotCount          int // etcd
	HeartbeatInterval      int // etcd
	ElectionTimeout        int // etcd
	QuotaBackendBytes      int // etcd
	MaxSnapshots           int // etcd
	MaxWals                int // etcd
	MdsPort                int // mds
	DummyPort              int // mds
	SnapshotClonePort      int // snapshotclone
	SnapshotCloneDummyPort int // snapshotclone
	SnapshotCloneProxyPort int // snapshotclone
	MetaserverPort         int // metaserver
	MetaserverExternalPort int // metaserver
	StandAlone             bool
}

func ConfigureNodeInfo(c *Cluster) ([]NodeInfo, error) {
	nodeNameIP, err := k8sutil.GetNodeInfoMap(c.Nodes, c.Context.Clientset)
	if err != nil {
		return nil, err
	}

	var (
		peerPort, clientPort                                              int
		snapshotCount, heartbeatInterval                                  int
		electionTimeout, quotaBackendBytes                                int
		maxSnapshots, maxWals                                             int
		mdsPort, dummyPort                                                int
		snapshotClonePort, snapshotCloneDummyPort, snapshotCloneProxyPort int
		metaserverPort, metaserverExternalPort                            int
		prevNodeName                                                      string
		nodesInfo                                                         []NodeInfo
	)
	hostID, replicasSequence := -1, -1
	standAlone := false
	// The order of node has been determined
	for _, node := range nodeNameIP {
		hostID++
		if node.NodeName == prevNodeName {
			standAlone = true
			replicasSequence++
			peerPort++
			clientPort++
			snapshotCount++
			heartbeatInterval++
			electionTimeout++
			quotaBackendBytes++
			maxSnapshots++
			maxWals++
			mdsPort++
			dummyPort++
			snapshotClonePort++
			snapshotCloneDummyPort++
			snapshotCloneProxyPort++
			metaserverPort++
			metaserverExternalPort++
		} else {
			replicasSequence = 0
			peerPort = c.Etcd.PeerPort
			clientPort = c.Etcd.ClientPort
			snapshotCount = c.Etcd.Config["snapshot-count"]
			heartbeatInterval = c.Etcd.Config["heartbeat-interval"]
			electionTimeout = c.Etcd.Config["election-timeout"]
			quotaBackendBytes = c.Etcd.Config["quota-backend-bytes"]
			maxSnapshots = c.Etcd.Config["max-snapshots"]
			maxWals = c.Etcd.Config["max-wals"]
			mdsPort = c.Mds.Port
			dummyPort = c.Mds.DummyPort
			snapshotClonePort = c.SnapShotClone.Port
			snapshotCloneDummyPort = c.SnapShotClone.DummyPort
			snapshotCloneProxyPort = c.SnapShotClone.ProxyPort
			metaserverPort = c.Metaserver.Port
			metaserverExternalPort = c.Metaserver.ExternalPort
		}
		prevNodeName = node.NodeName
		nodesInfo = append(nodesInfo, NodeInfo{
			NodeName:               node.NodeName,
			NodeIP:                 node.NodeIP,
			HostID:                 hostID,
			ReplicasSequence:       replicasSequence,
			PeerPort:               peerPort,
			ClientPort:             clientPort,
			SnapshotCount:          snapshotCount,
			HeartbeatInterval:      heartbeatInterval,
			ElectionTimeout:        electionTimeout,
			QuotaBackendBytes:      quotaBackendBytes,
			MaxSnapshots:           maxSnapshots,
			MaxWals:                maxWals,
			MdsPort:                mdsPort,
			DummyPort:              dummyPort,
			SnapshotClonePort:      snapshotClonePort,
			SnapshotCloneDummyPort: snapshotCloneDummyPort,
			SnapshotCloneProxyPort: snapshotCloneProxyPort,
			MetaserverPort:         metaserverPort,
			MetaserverExternalPort: metaserverExternalPort,
			StandAlone:             standAlone,
		})
	}
	return nodesInfo, nil
}
