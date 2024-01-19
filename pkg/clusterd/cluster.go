package clusterd

import (
	curvev1 "github.com/opencurve/curve-operator/api/v1"
)

type Clusterer interface {
	GetContext() Context

	GetName() string
	GetNameSpace() string
	GetUUID() string
	GetKind() string
	GetOwnerInfo() *OwnerInfo

	GetContainerImage() string
	GetNodes() []string
	GetDataDir() string
	GetLogDir() string
	GetCopysets() int
	GetEtcdSpec() *curvev1.EtcdSpec
	GetMdsSpec() *curvev1.MdsSpec
	GetChunkserverSpec() *curvev1.StorageScopeSpec
	GetMetaserverSpec() *curvev1.MetaServerSpec
	GetSnapShotSpec() *curvev1.SnapShotCloneSpec

	GetRoleInstances(role string) int
	GetRolePort(role string) int
	GetRoleClientPort(role string) int
	GetRoleDummyPort(role string) int
	GetRoleProxyPort(role string) int
	GetRoleExternalPort(role string) int
	GetRoleConfigs(role string) map[string]string
}
