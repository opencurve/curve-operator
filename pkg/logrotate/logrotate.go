package logrotate

import (
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/daemon"
)

func CreateLogrotateConfigMap(c *daemon.Cluster) error {

	logrotateConfMapData := `/logs/* {
		rotate 5
		missingok
		compress
		copytruncate
		dateext
		createolddir
		olddir /logs/old
		size 10m
		notifempty
	}`
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "log-conf",
			Namespace: c.Namespace,
		},
		Data: map[string]string{
			"logrotate.conf": logrotateConfMapData,
		},
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create %s configmap in namespace %s", cm.Name, c.Namespace)
	}

	return nil
}

func MakeLogrotateContainer() v1.Container {
	container := v1.Container{
		Name:  "logrotate",
		Image: "linkyard/logrotate:1.0.0",
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "log-volume",
				MountPath: "/logs",
			},
			{
				Name:      "log-conf",
				MountPath: "/etc/logrotate.conf",
				SubPath:   "logrotate.conf",
			},
		},
	}
	return container
}
