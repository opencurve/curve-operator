package chunkserver

import (
	"context"
	"time"

	"github.com/coreos/pkg/capnslog"
	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	AppName = "curve-chunkserver"

	// ContainerPath is the mount path of data and log
	ChunkserverContainerDataDir = "/curvebs/chunkserver/data"
	ChunkserverContainerLogDir  = "/curvebs/chunkserver/logs"

	// start.sh
	startChunkserverConfigMapName     = "start-chunkserver-conf"
	startChunkserverScriptFileDataKey = "start_chunkserver.sh"
	startChunkserverMountPath         = "/curvebs/tools/sbin/start_chunkserver.sh"
)

type Cluster struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	spec           curvev1.CurveClusterSpec
}

var log = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "chunkserver")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec) *Cluster {
	return &Cluster{
		context:        context,
		namespacedName: namespacedName,
		spec:           spec,
	}
}

// Start begins the chunkserver daemon
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	log.Infof("start running chunkserver in namespace %q", c.namespacedName.Namespace)

	if !c.spec.Storage.UseSelectedNodes && (len(c.spec.Storage.Nodes) == 0 || len(c.spec.Storage.Devices) == 0) {
		log.Error("useSelectedNodes is set to false but no node or device specified")
		return errors.New("useSelectedNodes is set to false but no node specified")
	}

	if c.spec.Storage.UseSelectedNodes && len(c.spec.Storage.SelectedNodes) == 0 {
		log.Error("useSelectedNodes is set to false but selectedNodes not be specified")
		return errors.New("useSelectedNodes is set to false but selectedNodes not be specified")
	}

	log.Info("starting to prepare the chunk file")

	// 1. startProvisioningOverNodes format device and prepare chunk files
	err := c.startProvisioningOverNodes(nodeNameIP)
	if err != nil {
		log.Error("failed to prepare chunkfiles")
		return errors.Wrap(err, "failed to provision chunkfilepool")
	}

	// 2. wait all job finish to complete format and wait MDS election success.
	halfMinuteTicker := time.NewTicker(10 * time.Second)
	defer halfMinuteTicker.Stop()

	chn := make(chan bool, 1)
	ctx, canf := context.WithTimeout(context.Background(), time.Duration(10*60*time.Second))
	defer canf()
	c.checkJobStatus(ctx, halfMinuteTicker, chn)

	// block here unitl timeout(10 mins) or all jobs has been successed.
	flag := <-chn

	// not all job has completed
	if !flag {
		// TODO: delete all jobs that has created.
		log.Error("Format job is not completed in 10 minutes and exit with -1")
		return errors.New("Format job is not completed in 10 minutes and exit with -1")
	}

	log.Info("all jobs run completed in 10 mins")

	// 2. create physical pool
	_, err = c.runCreatePoolJob(nodeNameIP, "physical_pool")
	if err != nil {
		log.Error("failed to create physical pool")
		return errors.Wrap(err, "failed to create physical pool")
	}

	// 3. startChunkServers start all chunkservers for each device of every node
	err = c.startChunkServers()
	if err != nil {
		log.Error("failed to start chunkserver")
		return errors.Wrap(err, "failed to start chunkserver")
	}

	// 4. wait all chunkservers online before create logical pool
	time.Sleep(30 * time.Second)

	// 5. create logical pool
	_, err = c.runCreatePoolJob(nodeNameIP, "logical_pool")
	if err != nil {
		log.Error("failed to create logical pool")
		return errors.Wrap(err, "failed to create physical pool")
	}

	return nil
}

// checkJobStatus go routine to check all job's status
func (c *Cluster) checkJobStatus(ctx context.Context, ticker *time.Ticker, chn chan bool) {
	for {
		select {
		case <-ticker.C:
			log.Info("time is up")
			completed := 0
			for _, jobName := range jobsArr {
				job, err := c.context.Clientset.BatchV1().Jobs(c.namespacedName.Namespace).Get(jobName, metav1.GetOptions{})
				if err != nil {
					log.Errorf("failed to get job %s in cluster", jobName)
					return
				}

				if job.Status.Succeeded > 0 {
					completed++
					log.Infof("job %s has successd", job.Name)
				} else {
					log.Infof("job %s is running", job.Name)
				}

				if completed == len(jobsArr) {
					log.Info("all job has been successd, exit go routine")
					chn <- true
					return
				}
			}
		case <-ctx.Done():
			chn <- false
			log.Error("go routinue exit because check time is more than 5 mins")
			return
		}
	}
}
