package snapshotclone

var START = `

argsNginxConf=$1
argsSnapShotCloneAddr=$2
argsConfigFileDir=$3

/sur/sbin/nginx -c $argsNginxConf

# sleep 5 second to wait nginx startup
sleep 5

cd /curvebs/snapshotclone/sbin 
./curvebs-snapshotclone $2 $3
`
