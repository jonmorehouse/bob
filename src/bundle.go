package main

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// buildBundle: build a bundle by looking for and executing a `/build`
// executable script inside a docker container, and exporting the resultant
// `/output` directory
func buildBundle(builderConfig *BuilderConfig, projectConfig *ProjectConfig, buildConfig *BuildConfig) error {
	// create temporary directory
	safeName := strings.Replace(projectConfig.Name, "/", "--", -1)
	tempDir := fmt.Sprintf("/tmp/%s--%s", safeName, projectConfig.GitShortRef)
	if _, err := os.Stat(tempDir); err == nil {
		if err := os.RemoveAll(tempDir); err != nil {
			return err
		}
	}

	if err := os.Mkdir(tempDir, 0755); err != nil {
		return err
	}

	tag := fmt.Sprintf("builder/%s:%s", projectConfig.Name, projectConfig.GitShortRef)
	args := fmt.Sprintf("build -f %s -t %s .", buildConfig.Dockerfile, tag)
	if _, err := runCommand("docker", strings.Split(args, " ")); err != nil {
		return err
	}

	// if a `build` script exists, attempt to execute it, erring out when it doesn't exist
	if _, err := os.Stat(path.Join(projectConfig.Dir, "build")); err == nil {
		args = fmt.Sprintf("run -v %s:/output -t %s /build", tempDir, tag)
		if _, err := runCommand("docker", strings.Split(args, " ")); err != nil {
			return err
		}
	}

	if !projectConfig.Push {
		return nil
	}

	args = fmt.Sprintf("-dir %s -version %s -latest=true -project %s -gcs-prefix %s -url-prefix %s",
		tempDir,
		projectConfig.GitShortRef,
		projectConfig.Name,
		builderConfig.ArtifactorGCSPrefix,
		builderConfig.ArtifactorURLPrefix)

	if _, err := runCommand("artifactor", strings.Split(args, " ")); err != nil {
		return err
	}

	return nil
}
