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

	configMapData := make(map[string]string)
	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue

		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		configMapData[key] = value
	}

	if err := scanner.Err(); err != nil {
		return make(map[string]string), errors.Wrap(err, "failed to scan file")
	}

	return configMapData, nil
}

func ReadEtcdTypeConfig(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %q", path)
	}
	defer file.Close()

	configMapData := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		ret := strings.Split(line, ":")
		if len(ret) == 2 {
			key := strings.TrimSpace(ret[0])
			value := strings.TrimSpace(ret[1])
			configMapData[key] = value
		} else if len(ret) == 1 {
			configMapData[ret[0]] = ""
		} else if len(ret) > 2 {
			key := strings.TrimSpace(ret[0])
			value := ""
			for i := 1; i < len(ret); i++ {
				value = value + ":" + ret[i]
			}
			value = strings.TrimLeft(value, ":")
			value = strings.TrimSpace(value)
			configMapData[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to scan file")
	}
	return configMapData, nil
}

func ReadNginxConf(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		logger.Error("failed to read nginx config file")
		return nil, errors.Wrap(err, "failed to read nginx config file")
	}
	nginxData := make(map[string]string)
	nginxData["nginx.conf"] = string(content)
	return nginxData, nil
}
