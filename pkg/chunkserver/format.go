package chunkserver

var FORMAT = `
device_name=$1
device_mount_path=$2
percent=$3
chunkfile_size=$4
chunkfile_pool_dir=$5
chunkfile_pool_meta_path=$6

mkfs.ext4 $device_name
mount $device_name $device_mount_path

cd /curvebs/tools/sbin

./curve_format \
  -allocatePercent=$percent \
  -fileSize=$chunkfile_size \
  -filePoolDir=$chunkfile_pool_dir \
  -filePoolMetaPath=$chunkfile_pool_meta_path \
  -fileSystemPath=$chunkfile_pool_dir
`
