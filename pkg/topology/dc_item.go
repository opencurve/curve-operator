package topology

import "path"

const (
	REQUIRE_ANY = iota
	REQUIRE_INT
	REQUIRE_STRING
	REQUIRE_BOOL
	REQUIRE_POSITIVE_INTEGER

	// default value
	DEFAULT_REPORT_USAGE                    = true
	DEFAULT_CURVEBS_CONTAINER_IMAGE         = "opencurvedocker/curvebs:latest"
	DEFAULT_CURVEFS_CONTAINER_IMAGE         = "opencurvedocker/curvefs:latest"
	DEFAULT_ETCD_LISTEN_PEER_PORT           = 2380
	DEFAULT_ETCD_LISTEN_CLIENT_PORT         = 2379
	DEFAULT_MDS_LISTEN_PORT                 = 6700
	DEFAULT_MDS_LISTEN_DUMMY_PORT           = 7700
	DEFAULT_CHUNKSERVER_LISTN_PORT          = 8200
	DEFAULT_SNAPSHOTCLONE_LISTEN_PORT       = 5555
	DEFAULT_SNAPSHOTCLONE_LISTEN_DUMMY_PORT = 8081
	DEFAULT_SNAPSHOTCLONE_LISTEN_PROXY_PORT = 8080
	DEFAULT_METASERVER_LISTN_PORT           = 6800
	DEFAULT_METASERVER_LISTN_EXTARNAL_PORT  = 7800
	DEFAULT_ENABLE_EXTERNAL_SERVER          = false
	DEFAULT_CHUNKSERVER_COPYSETS            = 100 // copysets per chunkserver
	DEFAULT_METASERVER_COPYSETS             = 100 // copysets per metaserver
)

type (
	// config item
	item struct {
		key          string
		require      int
		exclude      bool        // exclude for service config
		defaultValue interface{} // nil means no default value
	}

	itemSet struct {
		items    []*item
		key2item map[string]*item
	}
)

// you should add config item to itemset iff you want to:
//   (1) check the configuration item value, like type, valid value OR
//   (2) filter out the configuration item for service config OR
//   (3) set the default value for configuration item
var (
	itemset = &itemSet{
		items:    []*item{},
		key2item: map[string]*item{},
	}

	CONFIG_PREFIX = itemset.insert(
		"Prefix",
		REQUIRE_STRING,
		true,
		func(dc *DeployConfig) interface{} {
			if dc.GetKind() == KIND_CURVEBS {
				return path.Join(LAYOUT_CURVEBS_ROOT_DIR, dc.GetRole())
			}
			return path.Join(LAYOUT_CURVEFS_ROOT_DIR, dc.GetRole())
		},
	)

	CONFIG_CONTAINER_IMAGE = itemset.insert(
		"ContainerImage",
		REQUIRE_STRING,
		true,
		func(dc *DeployConfig) interface{} {
			if dc.GetKind() == KIND_CURVEBS {
				return DEFAULT_CURVEBS_CONTAINER_IMAGE
			}
			return DEFAULT_CURVEFS_CONTAINER_IMAGE
		},
	)

	CONFIG_LOG_DIR = itemset.insert(
		"LogDir",
		REQUIRE_STRING,
		true,
		nil,
	)

	CONFIG_DATA_DIR = itemset.insert(
		"DataDir",
		REQUIRE_STRING,
		true,
		nil,
	)

	CONFIG_CORE_DIR = itemset.insert(
		"CoreDir",
		REQUIRE_STRING,
		true,
		nil,
	)

	CONFIG_LISTEN_PORT = itemset.insert(
		"Port",
		REQUIRE_POSITIVE_INTEGER,
		true,
		func(dc *DeployConfig) interface{} {
			switch dc.GetRole() {
			case ROLE_ETCD:
				return DEFAULT_ETCD_LISTEN_PEER_PORT
			case ROLE_MDS:
				return DEFAULT_MDS_LISTEN_PORT
			case ROLE_CHUNKSERVER:
				return DEFAULT_CHUNKSERVER_LISTN_PORT
			case ROLE_SNAPSHOTCLONE:
				return DEFAULT_SNAPSHOTCLONE_LISTEN_PORT
			case ROLE_METASERVER:
				return DEFAULT_METASERVER_LISTN_PORT
			}
			return nil
		},
	)

	CONFIG_LISTEN_CLIENT_PORT = itemset.insert(
		"ClientPort",
		REQUIRE_POSITIVE_INTEGER,
		true,
		DEFAULT_ETCD_LISTEN_CLIENT_PORT,
	)

	CONFIG_LISTEN_DUMMY_PORT = itemset.insert(
		"DummyPort",
		REQUIRE_POSITIVE_INTEGER,
		true,
		func(dc *DeployConfig) interface{} {
			switch dc.GetRole() {
			case ROLE_MDS:
				return DEFAULT_MDS_LISTEN_DUMMY_PORT
			case ROLE_SNAPSHOTCLONE:
				return DEFAULT_SNAPSHOTCLONE_LISTEN_DUMMY_PORT
			}
			return nil
		},
	)

	CONFIG_LISTEN_PROXY_PORT = itemset.insert(
		"ProxyPort",
		REQUIRE_POSITIVE_INTEGER,
		true,
		DEFAULT_SNAPSHOTCLONE_LISTEN_PROXY_PORT,
	)

	CONFIG_LISTEN_EXTERNAL_IP = itemset.insert(
		"ExternalIp",
		REQUIRE_STRING,
		true,
		func(dc *DeployConfig) interface{} {
			return dc.GetHost()
		},
	)

	CONFIG_LISTEN_EXTERNAL_PORT = itemset.insert(
		"ExternalPort",
		REQUIRE_POSITIVE_INTEGER,
		true,
		func(dc *DeployConfig) interface{} {
			if dc.GetRole() == ROLE_METASERVER {
				return DEFAULT_METASERVER_LISTN_EXTARNAL_PORT
			}
			return dc.GetListenPort()
		},
	)

	CONFIG_ENABLE_EXTERNAL_SERVER = itemset.insert(
		"global.enable_external_server",
		REQUIRE_BOOL,
		false,
		DEFAULT_ENABLE_EXTERNAL_SERVER,
	)

	CONFIG_COPYSETS = itemset.insert(
		"Copysets",
		REQUIRE_POSITIVE_INTEGER,
		true,
		func(dc *DeployConfig) interface{} {
			if dc.GetRole() == ROLE_CHUNKSERVER {
				return DEFAULT_CHUNKSERVER_COPYSETS
			}
			return DEFAULT_METASERVER_COPYSETS
		},
	)

	CONFIG_S3_ACCESS_KEY = itemset.insert(
		"s3.ak",
		REQUIRE_STRING,
		false,
		nil,
	)

	CONFIG_S3_SECRET_KEY = itemset.insert(
		"s3.sk",
		REQUIRE_STRING,
		false,
		nil,
	)

	CONFIG_S3_ADDRESS = itemset.insert(
		"s3.nos_address",
		REQUIRE_STRING,
		false,
		nil,
	)

	CONFIG_S3_BUCKET_NAME = itemset.insert(
		"s3.snapshot_bucket_name",
		REQUIRE_STRING,
		false,
		nil,
	)
)

func (i *item) Key() string {
	return i.key
}

func (itemset *itemSet) insert(key string, require int, exclude bool, defaultValue interface{}) *item {
	i := &item{key, require, exclude, defaultValue}
	itemset.key2item[key] = i
	itemset.items = append(itemset.items, i)
	return i
}

func (itemset *itemSet) get(key string) *item {
	return itemset.key2item[key]
}

// func (itemset *itemSet) getAll() []*item {
// 	return itemset.items
// }
