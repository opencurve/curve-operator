package service

var wait_mds_election string = `
#!/usr/bin/env bash

[[ -z $(which curl) ]] && apt-get install -y curl
wait=0
while ((wait<20))
do
	for addr in "$CLUSTER_MDS_ADDR"
	do
		curl --connect-timeout 3 --max-time 10 $addr -Iso /dev/null
		if [ $? == 0 ]; then
		   exit 0
		fi
	done
	sleep 0.5s
	wait=$(expr $wait + 1)
done

exit 1
`

var wait_chunkserver_start string = `
#!/usr/bin/env bash

g_total=${CHUNKSERVER_NUMS}
total=$(expr $g_total + 0)

wait=0
while ((wait<60))
do
    online=$(curve_ops_tool chunkserver-status | sed -nr 's/.*online = ([0-9]+).*/\1/p')
    if [[ $online -eq $total ]]; then
        exit 0
    fi

    sleep 0.5s
    wait=$((wait+1))
done

exit 1
`
