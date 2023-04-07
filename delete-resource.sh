# etcd configmap
kubectl delete cm curve-etcd-config-a -n curvebs
kubectl delete cm curve-etcd-config-b -n curvebs
kubectl delete cm curve-etcd-config-c -n curvebs
kubectl delete cm curve-etcd-conf -n curvebs

# override configmap
kubectl delete cm etcd-endpoints-override -n curvebs
# kubectl delete cm mds-endpoints-override -n curvebs

# chunkserver 4 configmap
kubectl delete cm format-chunkfile-conf -n curvebs

kubectl delete cm curve-chunkserver-conf -n curvebs
kubectl delete cm cs-client-conf -n curvebs
kubectl delete cm s3-conf -n curvebs
kubectl delete cm start-chunkserver-conf -n curvebs
kubectl delete cm topology-json-conf -n curvebs
kubectl delete cm tools-conf -n curvebs
kubectl delete cm mds-endpoints-override -n curvebs
kubectl delete cm snapshotclone-conf -n curvebs
kubectl delete cm snap-client-conf -n curvebs
kubectl delete cm nginx-conf -n curvebs
kubectl delete cm start-snap-conf -n curvebs

kubectl delete cm chunkserver-conf-template -n curvebs
kubectl delete cm client-conf-template -n curvebs
kubectl delete cm cs-conf-template -n curvebs
kubectl delete cm start-snap-conf -n curvebs
kubectl delete cm start-snap-conf -n curvebs
kubectl delete cm start-snap-conf -n curvebs
kubectl delete cm start-snap-conf -n curvebs


# etcd deployment 
kubectl delete deploy curve-etcd-a -n curvebs
kubectl delete deploy curve-etcd-b -n curvebs
kubectl delete deploy curve-etcd-c -n curvebs

# mds configmap
kubectl delete cm curve-mds-config -n curvebs

# mds deployment
kubectl delete deploy curve-mds-a -n curvebs
kubectl delete deploy curve-mds-b -n curvebs
kubectl delete deploy curve-mds-c -n curvebs

# chunkserver deployment
kubectl delete deploy curve-chunkserver-node1-sdb -n curvebs
kubectl delete deploy curve-chunkserver-node2-sdb -n curvebs

# snapshotclone deployment
kubectl delete deploy curve-snapshotclone-a -n curvebs
kubectl delete deploy curve-snapshotclone-b -n curvebs
kubectl delete deploy curve-snapshotclone-c -n curvebs



# all job in curvebs cluster
kubectl delete --all job -n curvebs
# all deploy in curvebs cluster
kubectl delete --all deployment -n curvebs

# all po in curvebs cluster
kubectl delete --all pods -n curvebs

# curvecluster cr
kubectl delete -f config/samples/
