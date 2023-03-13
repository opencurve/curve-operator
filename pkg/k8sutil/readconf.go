package k8sutil

import (
	"bufio"
	"os"
	"strings"

	"github.com/pkg/errors"
)

// ReadConfFromTemplate read config file of each daemon to map.
func ReadConfFromTemplate(path string) (map[string]string, error) {
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
