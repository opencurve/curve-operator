package controllers

import (
	"strings"

	"github.com/opencurve/curve-operator/pkg/config"
)

// truncateConfigName get template configmap
func truncateConfigName(configName string) string {
	tmpStr := "-conf-template"
	var configMapName string
	s := strings.Split(configName, ".")
	if len(strings.Split(s[0], "_")) > 1 {
		configMapName = strings.Split(s[0], "_")[0] + tmpStr
	} else {
		configMapName = s[0] + tmpStr
	}
	return configMapName
}

// ParseConfigByDelimiter parses a config file according to different delimiter
func parseConfigByDelimiter(content string, delimiter string) (map[string]string, error) {
	lines := strings.Split(content, "\n")
	config := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" { // skip the comment lines and blank lines
			continue
		}
		parts := strings.SplitN(line, delimiter, 2)
		if len(parts) != 2 {
			continue // ignore invalid line
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		config[key] = value
	}
	return config, nil
}

// parseNginxConf parse nginx.conf and return map
func parseNginxConf(content string) (map[string]string, error) {
	nginxData := make(map[string]string)
	nginxData[config.NginxConfigMapDataKey] = string(content)
	return nginxData, nil
}
