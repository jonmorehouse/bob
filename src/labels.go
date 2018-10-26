package main

import (
	"fmt"
	"strings"
	"time"
)

func getSecondEpochTS() int64 {
	return time.Now().Unix()
}

// uniquify: removes duplicates from the list
// buildLabels: generate a list of labels given the project and build configs
func buildLabels(projectConfig *ProjectConfig, buildConfig *BuildConfig) ([]string, error) {
	substitutions := map[string]string{
		"${SECOND_TIMESTAMP}": fmt.Sprintf("%d", getSecondEpochTS()),
		"${TIMESTAMP}":        fmt.Sprintf("%d", getSecondEpochTS()),
		"${GIT_REF}":          projectConfig.GitRef,
		"${GIT_SHORT_REF}":    projectConfig.GitShortRef,
	}

	labels := make([]string, 0, len(buildConfig.Labels))

	for k, v := range labels {
		for sub, value := range substitutions {
			v = strings.Replace(v, sub, value, -1)
		}

		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}

	return labels, nil
}
