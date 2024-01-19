package controllers

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
	"github.com/opencurve/curve-operator/pkg/utils"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	BS_RECORD_CONFIGMAP = "bs-record-config"
	FS_RECORD_CONFIGMAP = "fs-record-config"
)

var roles = []string{
	topology.ROLE_ETCD,
	topology.ROLE_MDS,
	topology.ROLE_CHUNKSERVER,
	topology.ROLE_SNAPSHOTCLONE,
	topology.ROLE_METASERVER,
}

func fmtParameter(k, v interface{}) string {
	return fmt.Sprintf("%s=%s", k, v)
}

func parseSpecParameters(cluster clusterd.Clusterer) (map[string]map[string]string, map[string]string) {
	parameters := map[string]map[string]string{}
	data := map[string]string{}

	for _, role := range roles {
		specRolePara := map[string]string{}
		roleParaLine := []string{}
		switch role {
		case topology.ROLE_ETCD:
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.CLIENT_PORT, *cluster.GetEtcdSpec().ClientPort))
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.PEER_PORT, *cluster.GetEtcdSpec().PeerPort))
			specRolePara[curvev1.CLIENT_PORT] = strconv.Itoa(*cluster.GetEtcdSpec().ClientPort)
			specRolePara[curvev1.PEER_PORT] = strconv.Itoa(*cluster.GetEtcdSpec().PeerPort)
		case topology.ROLE_MDS:
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.PORT, *cluster.GetMdsSpec().Port))
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.DUMMY_PORT, *cluster.GetMdsSpec().DummyPort))
			specRolePara[curvev1.PORT] = strconv.Itoa(*cluster.GetMdsSpec().Port)
			specRolePara[curvev1.DUMMY_PORT] = strconv.Itoa(*cluster.GetMdsSpec().DummyPort)

		}
		if role == topology.ROLE_CHUNKSERVER && cluster.GetKind() == topology.KIND_CURVEBS {
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.PORT, *cluster.GetChunkserverSpec().Port))
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.INSTANCES, cluster.GetChunkserverSpec().Instances))
			specRolePara[curvev1.PORT] = strconv.Itoa(*cluster.GetChunkserverSpec().Port)
			specRolePara[curvev1.INSTANCES] = strconv.Itoa(cluster.GetChunkserverSpec().Instances)
		}

		if role == topology.ROLE_SNAPSHOTCLONE && cluster.GetKind() == topology.KIND_CURVEBS {
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.PORT, *cluster.GetSnapShotSpec().Port))
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.DUMMY_PORT, *cluster.GetSnapShotSpec().DummyPort))
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.PROXY_PORT, *cluster.GetSnapShotSpec().ProxyPort))
			specRolePara[curvev1.PORT] = strconv.Itoa(*cluster.GetSnapShotSpec().Port)
			specRolePara[curvev1.DUMMY_PORT] = strconv.Itoa(*cluster.GetSnapShotSpec().DummyPort)
			specRolePara[curvev1.PROXY_PORT] = strconv.Itoa(*cluster.GetSnapShotSpec().ProxyPort)
		}

		if role == topology.ROLE_METASERVER && cluster.GetKind() == topology.KIND_CURVEFS {
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.PORT, *cluster.GetMetaserverSpec().Port))
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.EXTERNAL_PORT, *cluster.GetMetaserverSpec().ExternalPort))
			roleParaLine = append(roleParaLine, fmtParameter(curvev1.INSTANCES, cluster.GetMetaserverSpec().Instances))
			specRolePara[curvev1.PORT] = strconv.Itoa(*cluster.GetMetaserverSpec().Port)
			specRolePara[curvev1.EXTERNAL_PORT] = strconv.Itoa(*cluster.GetMetaserverSpec().ExternalPort)
			specRolePara[curvev1.INSTANCES] = strconv.Itoa(cluster.GetMetaserverSpec().Instances)
		}

		for key, val := range cluster.GetRoleConfigs(role) {
			roleParaLine = append(roleParaLine, fmtParameter(key, val))
			specRolePara[key] = val
		}
		content := strings.Join(roleParaLine, "\n")
		data[role] = content
		parameters[role] = specRolePara
	}

	return parameters, data
}

func createorUpdateRecordConfigMap(cluster clusterd.Clusterer) error {
	configmapName := topology.Choose(
		cluster.GetKind() == topology.KIND_CURVEBS, BS_RECORD_CONFIGMAP, FS_RECORD_CONFIGMAP)
	_, mapStringData := parseSpecParameters(cluster)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: cluster.GetNameSpace(),
		},
		Data: mapStringData,
	}

	err := cluster.GetOwnerInfo().SetControllerReference(cm)
	if err != nil {
		return err
	}

	_, err = k8sutil.CreateOrUpdateConfigMap(cluster.GetContext().Clientset, cm)
	if err != nil {
		return err
	}

	return nil
}

// getDataFromRecordConfigMap reads data from the ConfigMap of the record and returns the data for formatting
func getDataFromRecordConfigMap(cluster clusterd.Clusterer) (map[string]map[string]string, error) {
	configmapName := topology.Choose(
		cluster.GetKind() == topology.KIND_CURVEBS, BS_RECORD_CONFIGMAP, FS_RECORD_CONFIGMAP)

	cm, err := k8sutil.GetConfigMapByName(cluster.GetContext().Clientset, cluster.GetNameSpace(), configmapName)
	if err != nil {
		return nil, err
	}
	allData := map[string]map[string]string{}
	for key, value := range cm.Data {
		oneroleConfig := map[string]string{}
		lines := strings.Split(value, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) == 0 || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.Split(line, "=")

			if len(parts) >= 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				oneroleConfig[key] = value
			}
		}

		allData[key] = oneroleConfig
	}

	return allData, nil
}

// constructTemplateConfigMap start a dummy Deployment service and read template config file to a ConfigMap
func constructConfigMap(cluster clusterd.Clusterer, dcs []*topology.DeployConfig) error {
	if err := makeDummyDeployment(cluster, dcs); err != nil {
		return err
	}

	if err := makeTemplateConfigMap(cluster, dcs); err != nil {
		return err
	}

	if _, err := makeMutateConfigMap(cluster); err != nil {
		return err
	}

	return nil
}

func makeMutateConfigMap(cluster clusterd.Clusterer) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.AFTER_MUTATE_CONF,
			Namespace: cluster.GetNameSpace(),
		},
		Data: map[string]string{},
	}

	err := cluster.GetOwnerInfo().SetControllerReference(cm)
	if err != nil {
		return nil, err
	}

	_, err = k8sutil.CreateOrUpdateConfigMap(cluster.GetContext().Clientset, cm)
	if err != nil {
		return nil, err
	}

	return cm, nil
}

func mutateConfig(cluster clusterd.Clusterer, dc *topology.DeployConfig, name string) error {
	templateCM, err := cluster.GetContext().Clientset.CoreV1().ConfigMaps(cluster.GetNameSpace()).Get(CURVE_CONFIG_TEMPLATE, metav1.GetOptions{})
	if err != nil {
		return err
	}
	afterMutateCM, err := cluster.GetContext().Clientset.CoreV1().ConfigMaps(cluster.GetNameSpace()).Get(utils.AFTER_MUTATE_CONF, metav1.GetOptions{})
	if err != nil {
		return err
	}

	input := templateCM.Data[name]

	var key, value string
	output := []string{}
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		in := scanner.Text()
		err := kvFilter(dc, in, &key, &value)
		if err != nil {
			return err
		}
		out, err := mutate(dc, in, key, value, name)
		if err != nil {
			return err
		}

		output = append(output, out)
	}
	content := strings.Join(output, "\n")
	afterKey := fmt.Sprintf("%s_%s", dc.GetName(), name)
	afterMutateCM.Data[afterKey] = content

	_, err = k8sutil.UpdateConfigMap(cluster.GetContext().Clientset, afterMutateCM)
	if err != nil {
		return err
	}

	return nil
}

func kvFilter(dc *topology.DeployConfig, line string, key, value *string) error {
	pattern := fmt.Sprintf(REGEX_KV_SPLIT, strings.TrimSpace(dc.GetConfigKvFilter()), dc.GetConfigKvFilter())
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return errors.New("failed to build regex")
	}

	mu := regex.FindStringSubmatch(line)
	if len(mu) == 0 {
		*key = ""
		*value = ""
	} else {
		*key = mu[2]
		*value = mu[3]
	}

	return nil
}

func mutate(dc *topology.DeployConfig, in, key, value string, name string) (out string, err error) {
	if len(key) == 0 {
		out = in
		if name == "nginx.conf" { // only for nginx.conf
			out, err = dc.GetVariables().Rendering(in)
		}
		return
	}

	// replace config
	v, ok := dc.GetServiceConfig()[strings.ToLower(key)]
	if ok {
		value = v
	}

	// replace variable
	value, err = dc.GetVariables().Rendering(value)
	if err != nil {
		return
	}

	return
}
