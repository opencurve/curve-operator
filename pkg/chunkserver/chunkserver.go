package chunkserver

import (
	"context"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	AppName             = "curve-chunkserver"
	ConfigMapNamePrefix = "curve-chunkserver-conf"

	// ContainerPath is the mount path of data and log
	Prefix                      = "/curvebs/chunkserver"
	ChunkserverContainerDataDir = "/curvebs/chunkserver/data"
	ChunkserverContainerLogDir  = "/curvebs/chunkserver/logs"

	// start.sh
	startChunkserverConfigMapName     = "start-chunkserver-conf"
	startChunkserverScriptFileDataKey = "start_chunkserver.sh"
	startChunkserverMountPath         = "/curvebs/tools/sbin/start_chunkserver.sh"
)

type Cluster struct {
	context         clusterd.Context
	namespacedName  types.NamespacedName
	spec            curvev1.CurveClusterSpec
	dataDirHostPath string
	logDirHostPath  string
	confDirHostPath string
	ownerInfo       *k8sutil.OwnerInfo
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "chunkserver")

func New(context clusterd.Context,
	namespacedName types.NamespacedName,
	spec curvev1.CurveClusterSpec,
	ownerInfo *k8sutil.OwnerInfo,
	dataDirHostPath string,
	logDirHostPath string,
	confDirHostPath string) *Cluster {
	return &Cluster{
		context:         context,
		namespacedName:  namespacedName,
		spec:            spec,
		dataDirHostPath: dataDirHostPath,
		logDirHostPath:  logDirHostPath,
		confDirHostPath: confDirHostPath,
		ownerInfo:       ownerInfo,
	}
}

// Start begins the chunkserver daemon
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	logger.Infof("start running chunkserver in namespace %q", c.namespacedName.Namespace)

	if !c.spec.Storage.UseSelectedNodes && (len(c.spec.Storage.Nodes) == 0 || len(c.spec.Storage.Devices) == 0) {
		return errors.New("useSelectedNodes is set to false but no node specified")
	}

	if c.spec.Storage.UseSelectedNodes && len(c.spec.Storage.SelectedNodes) == 0 {
		return errors.New("useSelectedNodes is set to false but selectedNodes not be specified")
	}

	logger.Info("starting to prepare the chunk file")

	// 1. startProvisioningOverNodes format device and prepare chunk files
	err := c.startProvisioningOverNodes(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to provision chunkfilepool")
	}

	// 2. wait all job finish to complete format and wait MDS election success.
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.namespacedName, curvev1.ConditionTypeFormatedReady, curvev1.ConditionTrue, curvev1.ConditionFormatingChunkfilePoolReason, "Formating chunkfilepool")
	halfMinuteTicker := time.NewTicker(1 * time.Minute)
	defer halfMinuteTicker.Stop()

	chn := make(chan bool, 1)
	ctx, canf := context.WithTimeout(context.Background(), time.Duration(24*60*60*time.Second))
	defer canf()
	c.checkJobStatus(ctx, halfMinuteTicker, chn)

	// block here unitl timeout(24 hours) or all jobs has been successed.
	flag := <-chn

	// not all job has completed
	if !flag {
		// TODO: delete all jobs that has created.
		return errors.New("Format job is not completed in 24 hours and exit with -1")
	}
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.namespacedName, curvev1.ConditionTypeFormatedReady, curvev1.ConditionTrue, curvev1.ConditionFormatChunkfilePoolReason, "Formating chunkfilepool successed")

	logger.Info("all jobs run completed in 24 hours")

	// 2. create physical pool
	_, err = c.runCreatePoolJob(nodeNameIP, "physical_pool")
	if err != nil {
		return errors.Wrap(err, "failed to create physical pool")
	}
	logger.Info("create physical pool successed")

	// 3. startChunkServers start all chunkservers for each device of every node
	err = c.startChunkServers()
	if err != nil {
		return errors.Wrap(err, "failed to start chunkserver")
	}

	// 4. wait all chunkservers online before create logical pool
	logger.Info("starting all chunkserver")
	time.Sleep(30 * time.Second)

	// 5. create logical pool
	_, err = c.runCreatePoolJob(nodeNameIP, "logical_pool")
	if err != nil {
		return errors.Wrap(err, "failed to create physical pool")
	}
	logger.Info("create logical pool successed")

	k8sutil.UpdateCondition(context.TODO(), &c.context, c.namespacedName, curvev1.ConditionTypeChunkServerReady, curvev1.ConditionTrue, curvev1.ConditionChunkServerClusterCreatedReason, "Chunkserver cluster has been created")

	return nil
}

// checkJobStatus go routine to check all job's status
func (c *Cluster) checkJobStatus(ctx context.Context, ticker *time.Ticker, chn chan bool) {
	for {
		select {
		case <-ticker.C:
			logger.Info("time is up(1 minute)")
			completed := 0
			for _, jobName := range jobsArr {
				job, err := c.context.Clientset.BatchV1().Jobs(c.namespacedName.Namespace).Get(jobName, metav1.GetOptions{})
				if err != nil {
					logger.Errorf("failed to get job %s in cluster", jobName)
					chn <- false
					return
				}

				if job.Status.Succeeded > 0 {
					completed++
					logger.Infof("job %s has successd", job.Name)
				} else {
					logger.Infof("job %s is running", job.Name)
				}

				if completed == len(jobsArr) {
					logger.Info("all job has been successd, exit go routine")
					chn <- true
					return
				}
			}
		case <-ctx.Done():
			chn <- false
			logger.Error("go routinue exit because check time is more than 5 mins")
			return
		}
	}
}
