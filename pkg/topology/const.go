package topology

const (
	KIND_CURVEBS = "curvebs"
	KIND_CURVEFS = "curvefs"
)

const (
	ROLE_ETCD          = "etcd"
	ROLE_MDS           = "mds"
	ROLE_CHUNKSERVER   = "chunkserver"
	ROLE_SNAPSHOTCLONE = "snapshotclone"
	ROLE_METASERVER    = "metaserver"
)

var (
	CURVEBS_ROLES = []string{
		ROLE_ETCD,
		ROLE_MDS,
		ROLE_CHUNKSERVER,
		ROLE_SNAPSHOTCLONE,
	}
	CURVEFS_ROLES = []string{
		ROLE_ETCD,
		ROLE_MDS,
		ROLE_METASERVER,
	}
)
