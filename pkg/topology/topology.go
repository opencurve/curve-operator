package topology

import (
	"fmt"
	"strconv"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/utils"

	"github.com/pkg/errors"
)

// ParseTopology parse topology according to BsCluster or FsCluster CR(yaml declaration)
func ParseTopology(cluster clusterd.Clusterer) ([]*DeployConfig, error) {
	kind := cluster.GetKind()
	roles := []string{}
	if kind == clusterd.KIND_CURVEBS {
		roles = append(roles, CURVEBS_ROLES...)
	} else if kind == clusterd.KIND_CURVEFS {
		roles = append(roles, CURVEFS_ROLES...)
	} else {
		return nil, errors.New("Unknown cluster kind")
	}

	dcs := []*DeployConfig{}
	for _, role := range roles {
		for hostSequence, host := range cluster.GetNodes() {
			instances := cluster.GetRoleInstances(role)
			// deploy etcd || mds || snapshotclone service using first three nodes
			if role == ROLE_ETCD || role == ROLE_MDS || role == ROLE_SNAPSHOTCLONE {
				if hostSequence > 2 {
					break
				}
			}
			hostIp, err := k8sutil.GetNodeIpByName(host, cluster.GetContext().Clientset)
			if err != nil {
				return nil, err
			}
			for instancesSequence := 0; instancesSequence < instances; instancesSequence++ {
				config := cluster.GetRoleConfigs(role)
				// merge port config and global config to configs of each service
				mergePortConfig(cluster, role, instancesSequence, config)
				mergeGlobalConfig(cluster, role, instancesSequence, config)
				dc, err := NewDeployConfig(kind, role, host, hostIp, instances,
					instancesSequence, hostSequence, config)
				if err != nil {
					return nil, err
				}
				dcs = append(dcs, dc)
			}
		}
	}

	for i, dc := range dcs {
		if err := AddServiceVariables(dcs, i); err != nil {
			return nil, err
		} else if err = AddClusterVariables(dcs, i); err != nil {
			return nil, err
		}
		// Add config to serviceConfig
		dc.convert()
	}

	return dcs, nil
}

func NewDeployConfig(kind, role, host, hostIp string,
	instances, instanceSequence, hostSequence int,
	config map[string]string) (*DeployConfig, error) {

	for k, v := range config {
		if strv, ok := utils.All2Str(v); ok {
			config[k] = strv
		} else {
			return nil, errors.New("Unsupport Configure value type")
		}
	}

	return &DeployConfig{
		kind:              kind,
		id:                formatId(role, host, hostSequence, instanceSequence),
		parentId:          formatId(role, host, hostSequence, 0),
		name:              formatName(role, hostSequence, instanceSequence),
		role:              role,
		host:              host,
		hostIp:            hostIp,
		hostSequence:      hostSequence,
		instances:         instances,
		instancesSequence: instanceSequence,
		variables:         NewVariables(),
		config:            config,
		serviceConfig:     map[string]string{},
	}, nil
}

// getPortConfigOfRole handle specified port of every service
func mergePortConfig(cluster clusterd.Clusterer, role string,
	instanceSequence int, configs map[string]string) {
	if isEmptyString(configs[CONFIG_LISTEN_PORT.key]) {
		configs[CONFIG_LISTEN_PORT.key] = strconv.Itoa(cluster.GetRolePort(role) + instanceSequence)
	}
	if isEmptyString(configs[CONFIG_LISTEN_CLIENT_PORT.key]) {
		configs[CONFIG_LISTEN_CLIENT_PORT.key] = strconv.Itoa(cluster.GetRoleClientPort(role) + instanceSequence)
	}
	if isEmptyString(configs[CONFIG_LISTEN_DUMMY_PORT.key]) {
		configs[CONFIG_LISTEN_DUMMY_PORT.key] = strconv.Itoa(cluster.GetRoleDummyPort(role) + instanceSequence)
	}
	if isEmptyString(configs[CONFIG_LISTEN_EXTERNAL_PORT.key]) {
		configs[CONFIG_LISTEN_EXTERNAL_PORT.key] = strconv.Itoa(cluster.GetRoleExternalPort(role) + instanceSequence)
	}
}

// mergeGlobalConfig handle global config, such as
// ContainerImage, dataDir, logDir, Copysets etc.
func mergeGlobalConfig(cluster clusterd.Clusterer, role string,
	instanceSequence int, configs map[string]string) {
	if isEmptyString(configs[CONFIG_CONTAINER_IMAGE.key]) {
		configs[CONFIG_CONTAINER_IMAGE.key] = cluster.GetContainerImage()
	}
	if isEmptyString(configs[CONFIG_COPYSETS.key]) {
		configs[CONFIG_COPYSETS.key] = strconv.Itoa(cluster.GetCopysets())
	}
	if isEmptyString(configs[CONFIG_DATA_DIR.key]) {
		configs[CONFIG_DATA_DIR.key] = fmt.Sprint(trimString(cluster.GetDataDir()), "/", role, instanceSequence)
	} else {
		dataDir := configs[CONFIG_LOG_DIR.key]
		configs[CONFIG_LOG_DIR.key] = fmt.Sprint(trimString(dataDir), "/", role, instanceSequence)
	}
	if isEmptyString(configs[CONFIG_LOG_DIR.key]) {
		configs[CONFIG_LOG_DIR.key] = fmt.Sprint(trimString(cluster.GetLogDir()), "/", role, instanceSequence)
	} else {
		logDir := configs[CONFIG_LOG_DIR.key]
		configs[CONFIG_LOG_DIR.key] = fmt.Sprint(trimString(logDir), "/", role, instanceSequence)
	}
}
