package k8sutil

import (
	"bufio"
	"os"
	"strings"

	"github.com/pkg/errors"
)

// ReadConfFromTemplate read config file of each daemon to map.
func ReadConf(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return make(map[string]string), errors.Wrapf(err, "failed to open file %s ", path)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	configMapData := make(map[string]string)

	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || len(line) == 0 {
			continue
		}
		ret := strings.Split(line, "=")
		configMapData[ret[0]] = ret[1]
	}

	if err := scanner.Err(); err != nil {
		return make(map[string]string), errors.Wrap(err, "failed to scan file")
	}

	return configMapData, nil
}

func ReadEtcdTypeConfig(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return make(map[string]string), errors.Wrapf(err, "failed to open file %s ", path)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	configMapData := make(map[string]string)

	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || len(line) == 0 {
			continue
		}
		ret := strings.Split(line, ":")
		if len(ret) == 2 {
			ret[0] = strings.TrimSpace(ret[0])
			ret[1] = strings.TrimSpace(ret[1])
			configMapData[ret[0]] = ret[1]
		} else if len(ret) == 1 {
			configMapData[ret[0]] = ""
		} else if len(ret) > 2 {
			ret[0] = strings.TrimSpace(ret[0])
			tmp := ""
			for i := 1; i < len(ret); i++ {
				tmp = tmp + ":" + ret[i]
			}
			tmp = strings.TrimLeft(tmp, ":")
			tmp = strings.TrimSpace(tmp)
			configMapData[ret[0]] = tmp
		}
	}

	if err := scanner.Err(); err != nil {
		return make(map[string]string), errors.Wrap(err, "failed to scan file")
	}

	return configMapData, nil
}

func ReadNginxConf(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		logger.Error("failed to read nginx config file")
		return "", errors.Wrap(err, "failed to read nginx config file")
	}
	nginxStr := string(content)
	return nginxStr, nil
}
