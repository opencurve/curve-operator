package discover

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/util/sys"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var (
	// AppLabel is the app label
	AppLabel = "app"
	// AppName is the name of the pod
	AppName = "curve-discover"
	// NodeAttr is the attribute of that node
	NodeAttr = "curve.io/node"
	// LocalDiskCMData is the data name of the config map storing devices
	LocalDiskCMData = "devices"
	// LocalDiskCMName is name of the config map storing devices
	LocalDiskCMName = "local-device-%s"
	nodeName        string
	namespace       string
	lastDevice      string
	cmName          string
	cm              *v1.ConfigMap
	udevEventPeriod = time.Duration(5) * time.Second
)

func Run(ctx context.Context, context *clusterd.Context, probeInterval time.Duration) error {
	if context == nil {
		return fmt.Errorf("nil context")
	}
	klog.Infof("device discovery interval is %q", probeInterval.String())
	nodeName = os.Getenv(k8sutil.NodeNameEnvVar)
	namespace = os.Getenv(k8sutil.PodNamespaceEnvVar)
	cmName = fmt.Sprintf(LocalDiskCMName, nodeName)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM)

	err := updateDeviceCM(ctx, context)
	if err != nil {
		klog.Errorf("failed to update device configmap: %v", err)
		return err
	}

	udevEvents := make(chan struct{})
	go udevBlockMonitor(udevEvents, udevEventPeriod)
	for {
		select {
		case <-sigc:
			klog.Infof("shutdown signal received, exiting...")
			return nil
		case <-time.After(probeInterval):
			if err := updateDeviceCM(ctx, context); err != nil {
				klog.Errorf("failed to update device configmap during probe interval. %v", err)
			}
		case _, ok := <-udevEvents:
			if ok {
				klog.Info("trigger probe from udev event")
				if err := updateDeviceCM(ctx, context); err != nil {
					klog.Errorf("failed to update device configmap triggered from udev event. %v", err)
				}
			} else {
				klog.Warningf("disabling udev monitoring")
				udevEvents = nil
			}
		}
	}
}

func matchUdevEvent(text string, matches, exclusions []string) (bool, error) {
	for _, match := range matches {
		matched, err := regexp.MatchString(match, text)
		if err != nil {
			return false, fmt.Errorf("failed to search string: %v", err)
		}
		if matched {
			hasExclusion := false
			for _, exclusion := range exclusions {
				matched, err = regexp.MatchString(exclusion, text)
				if err != nil {
					return false, fmt.Errorf("failed to search string: %v", err)
				}
				if matched {
					hasExclusion = true
					break
				}
			}
			if !hasExclusion {
				klog.Infof("udevadm monitor: matched event: %s", text)
				return true, nil
			}
		}
	}
	return false, nil
}

// Scans `udevadm monitor` output for block sub-system events. Each line of
// output matching a set of substrings is sent to the provided channel. An event
// is returned if it passes any matches tests, and passes all exclusion tests.
func rawUdevBlockMonitor(c chan struct{}, matches, exclusions []string) {
	defer close(c)

	// stdbuf -oL performs line buffered output
	cmd := exec.Command("stdbuf", "-oL", "udevadm", "monitor", "-u", "-s", "block")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		klog.Warningf("Cannot open udevadm stdout: %v", err)
		return
	}
	defer stdout.Close()

	err = cmd.Start()
	if err != nil {
		klog.Warningf("Cannot start udevadm monitoring: %v", err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := scanner.Text()
		klog.Infof("udevadm monitor: %s", text)
		match, err := matchUdevEvent(text, matches, exclusions)
		if err != nil {
			klog.Warningf("udevadm filtering failed: %v", err)
			return
		}
		if match {
			c <- struct{}{}
		}
	}

	if err := scanner.Err(); err != nil {
		klog.Errorf("udevadm monitor scanner error: %v", err)
	}

	klog.Info("udevadm monitor finished")
}

// Monitors udev for block device changes, and collapses these events such that
// only one event is emitted per period in order to deal with flapping.
func udevBlockMonitor(c chan struct{}, period time.Duration) {
	defer close(c)
	var udevFilter []string

	// return any add or remove events, but none that match device mapper
	// events. string matching is case-insensitive
	events := make(chan struct{})

	blackList := getDiscoverUdevBlackList()
	udevFilter = strings.Split(blackList, ",")
	klog.Infof("using the regular expressions %q", udevFilter)

	go rawUdevBlockMonitor(events,
		[]string{"(?i)add", "(?i)remove"},
		udevFilter)

	timeout := time.NewTimer(period)
	defer timeout.Stop()
	for {
		_, ok := <-events
		if !ok {
			return
		}
		if !timeout.Stop() {
			<-timeout.C
		}
		timeout.Reset(period)
		for {
			select {
			case <-timeout.C:
			case _, ok := <-events:
				if !ok {
					return
				}
				continue
			}
			break
		}
		c <- struct{}{}
	}
}

func ignoreDevice(dev sys.LocalDisk) bool {
	return strings.Contains(strings.ToUpper(dev.DevLinks), "USB")
}

func checkMatchingDevice(checkDev sys.LocalDisk, devices []sys.LocalDisk) *sys.LocalDisk {
	for i, dev := range devices {
		if ignoreDevice(dev) {
			continue
		}
		// check if devices should be considered the same. the uuid can be
		// unstable, so we also use the reported serial and device name, which
		// appear to be more stable.
		if checkDev.UUID != "" && dev.UUID != "" && checkDev.UUID == dev.UUID {
			return &devices[i]
		}

		// on virt-io devices in libvirt, the serial is reported as an empty
		// string, so also account for that.
		if checkDev.Serial == dev.Serial && checkDev.Serial != "" {
			return &devices[i]
		}

		if checkDev.Name == dev.Name {
			return &devices[i]
		}
	}
	return nil
}

// note that the idea of equality here may not be intuitive. equality of device
// sets refers to a state in which no change has been observed between the sets
// of devices that would warrant changes to their consumption by storage
// daemons. for example, if a device appears to have been wiped vs a device
// appears to now be in use.
func checkDeviceListsEqual(oldDevs, newDevs []sys.LocalDisk) bool {
	for _, oldDev := range oldDevs {
		if ignoreDevice(oldDev) {
			continue
		}
		match := checkMatchingDevice(oldDev, newDevs)
		if match == nil {
			// device has been removed
			return false
		}
		if !oldDev.Empty && match.Empty {
			// device has changed from non-empty to empty
			return false
		}
		if oldDev.Partitions != nil && match.Partitions == nil {
			return false
		}
	}

	for _, newDev := range newDevs {
		if ignoreDevice(newDev) {
			continue
		}
		match := checkMatchingDevice(newDev, oldDevs)
		if match == nil {
			// device has been added
			return false
		}
		// the matching case is handled in the previous join
	}

	return true
}

// DeviceListsEqual checks whether 2 lists are equal or not
func DeviceListsEqual(old, new string) (bool, error) {
	var oldDevs []sys.LocalDisk
	var newDevs []sys.LocalDisk

	err := json.Unmarshal([]byte(old), &oldDevs)
	if err != nil {
		return false, fmt.Errorf("cannot unmarshal devices: %+v", err)
	}

	err = json.Unmarshal([]byte(new), &newDevs)
	if err != nil {
		return false, fmt.Errorf("cannot unmarshal devices: %+v", err)
	}

	return checkDeviceListsEqual(oldDevs, newDevs), nil
}

func updateDeviceCM(ctx context.Context, clusterdContext *clusterd.Context) error {
	klog.Infof("updating device configmap")
	devices, err := probeDevices(clusterdContext)
	if err != nil {
		klog.Errorf("failed to probe devices: %v", err)
		return err
	}
	deviceJSON, err := json.Marshal(devices)
	if err != nil {
		klog.Errorf("failed to marshal: %v", err)
		return err
	}

	deviceStr := string(deviceJSON)
	if cm == nil {
		cm, err = clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Get(cmName, metav1.GetOptions{})
	}
	if err == nil {
		lastDevice = cm.Data[LocalDiskCMData]
		klog.Infof("last devices %s", lastDevice)
	} else {
		if !kerrors.IsNotFound(err) {
			klog.Errorf("failed to get configmap: %v", err)
			return err
		}

		data := make(map[string]string, 1)
		data[LocalDiskCMData] = deviceStr

		// the map doesn't exist yet, create it now
		cm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: namespace,
				Labels: map[string]string{
					AppLabel: AppName,
					NodeAttr: nodeName,
				},
			},
			Data: data,
		}

		cm, err = clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Create(cm)
		if err != nil {
			klog.Errorf("failed to create configmap: %v", err)
			return fmt.Errorf("failed to create local device map %s: %+v", cmName, err)
		}
		lastDevice = deviceStr
	}
	devicesEqual, err := DeviceListsEqual(lastDevice, deviceStr)
	if err != nil {
		return fmt.Errorf("failed to compare device lists: %v", err)
	}
	if !devicesEqual {
		data := make(map[string]string, 1)
		data[LocalDiskCMData] = deviceStr
		cm.Data = data
		cm, err = clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Update(cm)
		if err != nil {
			klog.Errorf("failed to update configmap %s: %v", cmName, err)
			return err
		}
	}
	return nil
}

func logDevices(devices []*sys.LocalDisk) {
	var devicesList []string
	for _, device := range devices {
		klog.Infof("localdevice %q: %+v", device.Name, device)
		devicesList = append(devicesList, device.Name)
	}
	klog.Infof("localdevices: %q", strings.Join(devicesList, ", "))
}

func probeDevices(context *clusterd.Context) ([]sys.LocalDisk, error) {
	devices := make([]sys.LocalDisk, 0)
	blackList := getDiscoverUdevBlackList()
	localDevices, err := clusterd.DiscoverDevices(context.Executor, blackList)
	if err != nil {
		return devices, fmt.Errorf("failed initial hardware discovery. %+v", err)
	}

	logDevices(localDevices)

	for _, device := range localDevices {
		if device == nil {
			continue
		}

		partitions, _, err := sys.GetDevicePartitions(device.Name, context.Executor)
		if err != nil {
			klog.Errorf("failed to check device partitions %s: %v", device.Name, err)
			continue
		}

		// check if there is a file system on the device
		fs, err := sys.GetDeviceFilesystems(device.Name, context.Executor)
		if err != nil {
			klog.Errorf("failed to check device filesystem %s: %v", device.Name, err)
			continue
		}
		device.Partitions = partitions
		device.Filesystem = fs
		device.Empty = clusterd.GetDeviceEmpty(device)

		devices = append(devices, *device)
	}

	klog.Infof("available devices: %+v", devices)
	return devices, nil
}

func getDiscoverUdevBlackList() string {
	// get discoverDaemonUdevBlacklist from the environment variable
	// if user doesn't provide any regex; generate the default regex
	// else use the regex provided by user
	discoverUdev := os.Getenv(k8sutil.DiscoverUdevBlacklist)
	if discoverUdev == "" {
		// loop,fd0,sr0,/dev/ram*,/dev/dm-,/dev/md,/dev/rbd*,/dev/zd*
		discoverUdev = "(?i)loop[0-9]+,(?i)fd[0-9]+,(?i)sr[0-9]+,(?i)ram[0-9]+,(?i)dm-[0-9]+,(?i)md[0-9]+,(?i)zd[0-9]+,(?i)rbd[0-9]+,(?i)nbd[0-9]+"
	}
	return discoverUdev
}

func ReconcileDiscoveryDaemon() (err error) {
	clusterCtx := clusterd.NewContext()
	namespace := os.Getenv(k8sutil.PodNamespaceEnvVar)
	discoverImage := os.Getenv(k8sutil.OperatorImage)
	if EnableDiscoverDisk() {
		return Start(clusterCtx, namespace, discoverImage)
	} else {
		return Stop(clusterCtx, namespace)
	}
}

func EnableDiscoverDisk() bool {
	return os.Getenv(k8sutil.DiscoverDisk) == "true"
}

func Start(ctx *clusterd.Context, namespace, discoverImage string) error {
	err := createDiscoverDaemonSet(ctx, namespace, discoverImage)
	if err != nil {
		return fmt.Errorf("failed to start discover daemonset. %v", err)
	}
	return nil
}

func createDiscoverDaemonSet(ctx *clusterd.Context, namespace, discoverImage string) error {
	discoveryParameters := []string{"discover"}
	discoverInterval := os.Getenv(k8sutil.DiscoverInterval)
	if len(discoverInterval) > 0 {
		discoveryParameters = append(discoveryParameters, "--discover-interval",
			discoverInterval)
	}

	privileged := true
	ds := &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AppName,
			Namespace: namespace,
		},
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					AppLabel: AppName,
				},
			},
			UpdateStrategy: apps.DaemonSetUpdateStrategy{
				Type: apps.RollingUpdateDaemonSetStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						AppLabel: AppName,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "discover",
							Image: discoverImage,
							Args:  discoveryParameters,
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "dev",
									MountPath: "/dev",
									// discovery pod could fail to start if /dev is mounted ro
									ReadOnly: false,
								},
								{
									Name:      "sys",
									MountPath: "/sys",
									ReadOnly:  true,
								},
								{
									Name:      "udev",
									MountPath: "/run/udev",
									ReadOnly:  true,
								},
							},
							Env: []v1.EnvVar{
								k8sutil.NamespaceEnvVar(),
								k8sutil.NodeEnvVar(),
								k8sutil.NameEnvVar(),
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "dev",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/dev",
								},
							},
						},
						{
							Name: "sys",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/sys",
								},
							},
						},
						{
							Name: "udev",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/run/udev",
								},
							},
						},
					},
					HostNetwork: false,
				},
			},
		},
	}
	// Get the operator pod details to attach the owner reference to the discover daemon set
	operatorPod, err := k8sutil.GetRunningPod(ctx.Clientset)
	if err != nil {
		klog.Errorf("failed to get operator pod. %+v", err)
	} else {
		k8sutil.SetOwnerRefsWithoutBlockOwner(&ds.ObjectMeta, operatorPod.OwnerReferences)
	}

	_, err = ctx.Clientset.AppsV1().DaemonSets(namespace).Create(ds)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create rook-discover daemon set. %+v", err)
		}
		klog.Infof("rook-discover daemonset already exists, updating ...")
		_, err = ctx.Clientset.AppsV1().DaemonSets(namespace).Update(ds)
		if err != nil {
			return fmt.Errorf("failed to update rook-discover daemon set. %+v", err)
		}
	} else {
		klog.Infof("curve-discover daemonset started")
	}
	return nil

}

func Stop(ctx *clusterd.Context, namespace string) error {
	err := ctx.Clientset.AppsV1().DaemonSets(namespace).Delete(AppName, &metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	return nil
}
