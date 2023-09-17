#### English | [简体中文](https://github.com/opencurve/curve-operator/tree/master/docs/readme_cn.md)

# Curve-Operator

## What's curve-operator

curve-operator manages [Curve](https://github.com/opencurve/curve) cluster on [Kuberentes](https://kubernetes.io/docs/home/). It make Curve a truly cloud-native distributed storage system.

![Curve BS deploy architecture](https://github.com/opencurve/curve-operator/tree/master/docs/images/curve-deploy-arch.jpg)

## Setup

### 0.Prerequisite

* Kubernetes 1.19,1.20

### 1.Install Operator

The first step is to install the Curve Operator.

```shell
$ git clone https://github.com/opencurve/curve-operator.git
$ cd curve-operator
$ kubectl apply -f config/deploy/
```

verify the curve-operator on the `Running` state in `curve` namespace.

```shell
$ kubectl get pod -n curve
NAME                              READY   STATUS    RESTARTS   AGE
curve-operator-69bc69c75d-jfsjg   1/1     Running   0          7s
```

### 2. Deploy Curve Cluster

Operator deploys Curve cluster based on declarative API. You can get and modify customized the cluster yaml file in [config/sample.](https://github.com/opencurve/curve-operator/tree/master/config/samples)

[CurveBS Stand-alone deployment](https://github.com/opencurve/curve-operator/blob/master/config/samples/bscluster-onehost.yaml)

[CurveBS three replicas deployment](https://github.com/opencurve/curve-operator/blob/master/config/samples/cluster.yaml)

[CurveFS Stand-alone deployment](https://github.com/opencurve/curve-operator/blob/master/config/samples/fscluster-onehost.yaml)

[CurveFS three replicase deployment](https://github.com/opencurve/curve-operator/blob/master/config/samples/fscluster.yaml)

Here we take the deployment of a three-node `CurveBS` cluster as an example. The yaml file is [cluster.yaml](https://github.com/opencurve/curve-operator/blob/master/config/samples/cluster.yaml) and you can learn more about how to modify this configuration file through the comments in it.

Type the command to create the cluster：

```shell
$ kubectl apply -f config/samples/cluster.yaml
```

list all pods in the `curve` namespace:

```shell
$ kubectl -n curve get pod

NAME                                                          READY   STATUS      RESTARTS   AGE
curve-chunkserver-curve-operator-node1-vdc-556fc99467-5nx9q   1/1     Running     0          5m45s
curve-chunkserver-curve-operator-node2-vdc-7cf89768f9-hmcrs   1/1     Running     0          5m45s
curve-chunkserver-curve-operator-node3-vdc-f77dd85dc-z5bws    1/1     Running     0          5m45s
curve-etcd-a-d5bbfb755-lzgrm                                  1/1     Running     0          41m
curve-etcd-b-66c5b54f75-6nnnt                                 1/1     Running     0          41m
curve-etcd-c-86b7964f87-cj8zk                                 1/1     Running     0          41m
curve-mds-a-7b5989bddd-ln2sm                                  1/1     Running     0          40m
curve-mds-b-56d8f58645-gv6pd                                  1/1     Running     0          40m
curve-mds-c-997c7fd-vt5hw                                     1/1     Running     0          40m
gen-logical-pool-rzhlz                                        0/1     Completed   0          5m15s
gen-physical-pool-chnw8                                       0/1     Completed   0          5m45s
prepare-chunkfile-curve-operator-node1-vdc-znb66              0/1     Completed   0          40m
prepare-chunkfile-curve-operator-node2-vdc-6gf2z              0/1     Completed   0          40m
prepare-chunkfile-curve-operator-node3-vdc-2bkxm              0/1     Completed   0          40m
read-config-k272k                                             0/1     Completed   0          41m
```

> Tips: The chunkserver pods may not start immediately, because the disk needs to be formatted in the background(`prepare-chunkfile` jobs), so it may take a while to see the chunkserver pod. The waiting time is determined according to the number and percentage of configured disks, and it may be a long time.

### 3. Check cluster health

To verify that the cluster is in healthy state, enter one `curve-chunkserver` pod and type `curve_ops_tools status` command to check.

```shell
$ kubectl exec -it <any one chunkserver pod> -- bash
$ curve_ops_tool status

Cluster status:
cluster is healthy
total copysets: 100, unhealthy copysets: 0, unhealthy_ratio: 0%
physical pool number: 1, logical pool number: 1
Space info:
physical: total = 1178GB, used = 6GB(0.56%), left = 1171GB(99.44%)
logical: total = 392GB, used = 41GB(10.44%, can be recycled = 0GB(0.00%)), left = 351GB(89.56%), created file size = 60GB(15.28%)

Client status:
nebd-server: version-1.2.5+2c4861ca: 1
...
```

The cluster deployment completed and successfully if you see `cluster is healthy` prompt.

## Curve CSI

Create a PVC that to use curvebs as pod storage.

you can deploy and get more details from [curve-csi](https://github.com/opencurve/curve-csi) project that dock `curvebs` cluster or [curvefs-csi](https://github.com/opencurve/curvefs-csi) project that dock `curvefs` cluster.

## Remove

Remove curve cluster deployed already and clean up data on host.

### 1.Delete the cluster cr

```shell
$ kubectl -n curve delete curvecluster my-cluster
```

Verify the cluster CR has been deleted before continuing to the next step.

```shell
$ kubectl -n curve get curvecluster
```

### 2.Delete the Operator and related Resources

```shell
$ kubectl delete -f config/deploy/
```

### 3. Delete data and log on host

The final cleanup step requires deleting files on each host in the cluster. All files under the `hostDataDir` property specified in the cluster CRD will need to be deleted. Otherwise, inconsistent state will remain when a new cluster is started.

Connect to each machine and delete `/curvebs`, or the path specified by the `dataDirHostPath` and `logDirHostPath`.

```shell
$ rm -rf /curvebs
```

## Contributing

We welcome help in any form, including but not limited to improving documentation, asking questions, fixing bugs, and adding features. 

## Meeting

We have an online community meeting every two weeks which talk about what `Curve` is doing and planning to do. You can view meeting minutes and agenda here [Double Week Meetings](https://github.com/opencurve/curve-meetup-slides/tree/main/2023/Double%20Week%20Meetings).

## License

You are required to comply with the [CNCF](https://www.cncf.io/) Code of Conduct while participating in this project. 

## Contact

If you encounter any problems during use, please submit an [Issue](https://github.com/opencurve/curve-operator/issues) for feedback. You can also scan [the WeChat QR code](https://github.com/opencurve/curve-operator/tree/master/docs/images/curve-wechat.jpeg) to join the technical exchange group.
