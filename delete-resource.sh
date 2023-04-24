# etcd configmap
kubectl delete cm curve-etcd-conf-a -n curve
kubectl delete cm curve-etcd-conf-b -n curve
kubectl delete cm curve-etcd-conf-c -n curve
kubectl delete cm curve-etcd-conf -n curve

# override configmap
kubectl delete cm etcd-endpoints-override -n curve
# kubectl delete cm mds-endpoints-override -n curve

# chunkserver 4 configmap
kubectl delete cm format-chunkfile-conf -n curve

kubectl delete cm curve-chunkserver-conf -n curve
kubectl delete cm cs-client-conf -n curve
kubectl delete cm s3-conf -n curve
kubectl delete cm start-chunkserver-conf -n curve
kubectl delete cm topology-json-conf -n curve
kubectl delete cm tools-conf -n curve
kubectl delete cm mds-endpoints-override -n curve
kubectl delete cm snapshotclone-conf -n curve
kubectl delete cm snap-client-conf -n curve
kubectl delete cm nginx-conf -n curve
kubectl delete cm start-snap-conf -n curve

kubectl delete cm chunkserver-conf-template -n curve
kubectl delete cm client-conf-template -n curve
kubectl delete cm cs-conf-template -n curve
kubectl delete cm start-snap-conf -n curve
kubectl delete cm start-snap-conf -n curve
kubectl delete cm start-snap-conf -n curve
kubectl delete cm start-snap-conf -n curve


# etcd deployment 
kubectl delete deploy curve-etcd-a -n curve
kubectl delete deploy curve-etcd-b -n curve
kubectl delete deploy curve-etcd-c -n curve

# mds configmap
kubectl delete cm curve-mds-config -n curve

# mds deployment
kubectl delete deploy curve-mds-a -n curve
kubectl delete deploy curve-mds-b -n curve
kubectl delete deploy curve-mds-c -n curve

# chunkserver deployment
kubectl delete deploy curve-chunkserver-node1-sdb -n curve
kubectl delete deploy curve-chunkserver-node2-sdb -n curve

# snapshotclone deployment
kubectl delete deploy curve-snapshotclone-a -n curve
kubectl delete deploy curve-snapshotclone-b -n curve
kubectl delete deploy curve-snapshotclone-c -n curve



# all job in curve cluster
kubectl delete --all job -n curve
# all deploy in curve cluster
kubectl delete --all deployment -n curve

# all po in curve cluster
kubectl delete --all pods -n curve

# curvecluster cr
kubectl delete -f config/samples/
