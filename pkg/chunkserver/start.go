package chunkserver

var START = `
device_name=$1
device_mount_path=$2
argsChunkServerIp=$3
argsChunkServerExternalIp=$4
argsChunkFilePoolMetaPath=$5
argsWalFilePoolDir=$6
argsBthreadConcurrency=$7
argsRaftSyncSegments=$8
argsChunkserverPort=$9
argsChunkFilePoolDir=$10
argsRecycleUri=$11
argsChunkServerMetaUri=$12
argsWalFilePoolMetaPath=$13
argsRaftLogUri=$14
argsRaftSync=$15
argsRaftSyncMeta=$16
argsRaftMaxSegmentSize=$17
argsRaftMaxInstallSnapshotTasksNum=$18
argsRaftUseFsyncRatherThanFdatasync=$19
argsConf=$20
argsEnableExternalServer=$21
argsCopySetUri=$22
argsRaftSnapshotUri=$23
argsChunkServerStoreUri=$24
argsGracefulQuitOnSigterm=$25

echo $device_name
echo ${device_mount_path}

# argsChunkserverPort is the only int variable
# a node may be have more than one chunkserver, so port is must different
# such as 8200/8201/8202... 
echo ${argsChunkserverPort}

mkdir -p $device_mount_path
mount $device_name $device_mount_path

cd /curvebs/chunkserver/sbin

# for test
# while true; do echo hello; sleep 10;done

./curvebs-chunkserver -chunkServerIp=$3 \
  -chunkServerExternalIp=$4 \
  -chunkFilePoolMetaPath=$5 \
  -walFilePoolDir=$6 \
  -bthread_concurrency=$7 \
  -raft_sync_segments=$8 \
  -chunkServerPort=$9 \
  -chunkFilePoolDir=${10} \
  -recycleUri=${11} \
  -chunkServerMetaUri=${12} \
  -walFilePoolMetaPath=${13} \
  -raftLogUri=${14} \
  -raft_sync=${15} \
  -raft_sync_meta=${16} \
  -raft_max_segment_size=${17} \
  -raft_max_install_snapshot_tasks_num=${18} \
  -raft_use_fsync_rather_than_fdatasync=${19} \
  -conf=${20} \
  -enableExternalServer=${21} \
  -copySetUri=${22} \
  -raftSnapshotUri=${23} \
  -chunkServerStoreUri=${24} \
  -graceful_quit_on_sigterm=${25}
`
