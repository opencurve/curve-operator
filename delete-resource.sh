kubectl delete cm curve-etcd-config-a -n curvebs
kubectl delete cm curve-etcd-config-b -n curvebs
kubectl delete cm curve-etcd-config-c -n curvebs
kubectl delete deploy curve-etcd-a -n curvebs
kubectl delete deploy curve-etcd-b -n curvebs
kubectl delete deploy curve-etcd-c -n curvebs
kubectl delete -f config/samples/
