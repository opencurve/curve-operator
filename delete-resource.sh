# etcd configmap
kubectl delete cm curve-etcd-config-a -n curvebs
kubectl delete cm curve-etcd-config-b -n curvebs
kubectl delete cm curve-etcd-config-c -n curvebs

# override configmap
kubectl delete cm etcd-endpoints-override -n curvebs

# etcd deployment 
kubectl delete deploy curve-etcd-a -n curvebs
kubectl delete deploy curve-etcd-b -n curvebs
kubectl delete deploy curve-etcd-c -n curvebs

# mds configmap
kubectl delete cm curve-mds-config-a -n curvebs
kubectl delete cm curve-mds-config-b -n curvebs
kubectl delete cm curve-mds-config-c -n curvebs

# mds deployment
kubectl delete deploy curve-mds-a -n curvebs
kubectl delete deploy curve-mds-b -n curvebs
kubectl delete deploy curve-mds-c -n curvebs

# all po in curvebs cluster
kubectl delete --all pods -n curvebs

# curvecluster cr
kubectl delete -f config/samples/
