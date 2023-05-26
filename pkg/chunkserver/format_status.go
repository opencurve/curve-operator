package chunkserver

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type device2Use struct {
	nodeName      string
	deviceName    string
	devicePercent int
	status        string
	usePercent    int
}

// WaitForForamtJobCompletion waits for a format job to reach the completed state.
// only one pod in one job
func (c *Cluster) WaitForForamtJobCompletion(ctx context.Context, timeout time.Duration) error {
	retry := 0
	return wait.Poll(30*time.Second, timeout, func() (bool, error) {
		du, completed, err := c.getJob2DeviceFormatProgress()
		if err != nil {
			logger.Errorf("failed to get device format progress %v", err)
			return false, err
		}
		if completed {
			return true, nil
		}

		c.printProgress(retry, du)
		retry++

		return false, nil
	})
}

// getJobFormatStatus gets one device(one job) usage that represents format progress
func (c *Cluster) getJob2DeviceFormatProgress() ([]device2Use, bool, error) {
	device2UseArr := []device2Use{}
	completed := 0
	for _, watchedJob2DeviceInfo := range job2DeviceInfos {
		watchedJob := watchedJob2DeviceInfo.job
		watchedNodeName := watchedJob2DeviceInfo.nodeName
		wathedDevice := watchedJob2DeviceInfo.device
		job, err := c.Context.Clientset.BatchV1().Jobs(c.NamespacedName.Namespace).Get(context.Background(), watchedJob.Name, metav1.GetOptions{})
		if err != nil {
			return []device2Use{}, false, errors.Wrapf(err, "failed to get job %q in cluster", watchedJob.Name)
		}

		if job.Status.Succeeded > 0 {
			completed++
			if completed == len(job2DeviceInfos) {
				logger.Info("all format jobs has finished.")
				return device2UseArr, true, nil
			}
			continue
		}

		labels := c.getPodLabels(watchedNodeName, wathedDevice.Name)
		var labelSelector []string
		for k, v := range labels {
			labelSelector = append(labelSelector, k+"="+v)
		}
		selector := strings.Join(labelSelector, ",")
		podList, _ := c.Context.Clientset.CoreV1().Pods(watchedJob.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: selector,
		})
		if len(podList.Items) < 1 {
			// not occur
			logger.Warningf("no pod for job %q", watchedJob.Name)
			continue
		}

		// one job one pod one container
		pod := podList.Items[0]
		du, err := c.getDevUsedbyExecRequest(&pod, watchedNodeName, wathedDevice.Name, wathedDevice.Percentage, "Formatting")
		if err != nil {
			return []device2Use{}, false, errors.Wrap(err, "failed to get disk used percentage using exec request")
		}
		device2UseArr = append(device2UseArr, du)
	}

	return device2UseArr, false, nil
}

func (c *Cluster) getDevUsedbyExecRequest(pod *v1.Pod, nodeName, deviceName string, devicePercent int, status string) (device2Use, error) {
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)
	req := c.Context.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Container: pod.Spec.Containers[0].Name,
		Command:   []string{"df", "-h", deviceName},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.Context.KubeConfig, "POST", req.URL())
	if err != nil {
		return device2Use{}, fmt.Errorf("failed to init executor: %v", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})

	if err != nil {
		return device2Use{}, fmt.Errorf("could not execute: %v", err)
	}

	if execErr.Len() > 0 {
		return device2Use{}, fmt.Errorf("stderr: %v", execErr.String())
	}

	cmdOutput := execOut.String()
	re := regexp.MustCompile(`\S+\s+\S+\s+\S+\s+\S+\s+(?P<use>\d+)%`)
	use := 0
	match := re.FindStringSubmatch(cmdOutput)
	if len(match) > 1 {
		useStr := match[re.SubexpIndex("use")]
		use, err = strconv.Atoi(useStr)
		if err != nil {
			return device2Use{}, err
		}
	} else {
		logger.Info("Use value not found.")
	}

	if use > devicePercent {
		status = "Done"
	}
	deviceFormatInfo := device2Use{
		nodeName:      nodeName,
		deviceName:    deviceName,
		devicePercent: devicePercent,
		status:        status,
		usePercent:    use,
	}

	return deviceFormatInfo, nil
}

func (c *Cluster) printProgress(retry int, device2UseArr []device2Use) {
	if retry != 0 {
		fmt.Printf("\033[%dA", len(device2UseArr))
	}

	for _, device2Use := range device2UseArr {
		logger.Infof("node=%s\tdevice=%s\tformatted=%d/%d\tstatus=%s",
			device2Use.nodeName,
			device2Use.deviceName,
			device2Use.usePercent,
			device2Use.devicePercent,
			device2Use.status,
		)
	}
}
