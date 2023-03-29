package snapshotclone

var START = `

argsNginxConf=$1
argsConfigFileDir=$2

/usr/sbin/nginx -c $argsNginxConf

# sleep 5 second to wait nginx startup
sleep 10

# for test
#while true; do echo hello; sleep 10;done

cd /curvebs/snapshotclone/sbin 
./curvebs-snapshotclone $2
`
