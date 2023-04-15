package script

var START = `
device_name=$1
device_mount_path=$2
data_dir=$3
node_ip=$4
service_port=$5
conf_path=$6

mkdir -p $device_mount_path
mount $device_name $device_mount_path


# for test
# while true; do echo hello; sleep 10;done

cd /curvebs/chunkserver/sbin
./curvebs-chunkserver \
  -conf="${conf_path}" \
  -enableExternalServer=false \
  -copySetUri=local://"${data_dir}"/copysets \
  -raftLogUri=curve://"${data_dir}"/copysets \
  -raftSnapshotUri=curve://"${data_dir}"/copysets \
  -raft_sync_segments=true \
  -raft_max_install_snapshot_tasks_num=1 \
  -chunkServerIp=${node_ip} \
  -chunkFilePoolDir="${data_dir}" \
  -walFilePoolDir="${data_dir}" \
  -raft_sync=true \
  -raft_max_segment_size=8388608 \
  -raft_use_fsync_rather_than_fdatasync=false \
  -chunkFilePoolMetaPath="${data_dir}"/chunkfilepool.meta \
  -chunkServerStoreUri=local://"${data_dir}" \
  -chunkServerMetaUri=local://"${data_dir}"/chunkserver.dat \
  -bthread_concurrency=18 \
  -raft_sync_meta=true \
  -chunkServerExternalIp=${node_ip} \
  -chunkServerPort=${service_port} \
  -walFilePoolMetaPath="${data_dir}"/walfilepool.meta \
  -recycleUri=local://"${data_dir}"/recycler \
  -graceful_quit_on_sigterm=true
`
