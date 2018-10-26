package main

import (
	"fmt"
	"sync"
)

// buildDockerImage: build a docker image
func buildDockerImage(builderConfig *BuilderConfig, projectConfig *ProjectConfig, buildConfig *BuildConfig, tags []string) error {
	args := []string{"build", "-f", buildConfig.Dockerfile, "."}

	labels, err := buildLabels(projectConfig, buildConfig)
	if err != nil {
		return err
	}

	for _, label := range labels {
		args = append(args, "--label", label)
	}

	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	if _, err := runCommand("docker", args); err != nil {
		return err
	}

	return nil
}

// pushDockerImages: push images into their respective registries. Assumes that
// the docker daemon being used is authenticated.
func pushDockerImages(tags []string) error {
	var wg sync.WaitGroup

	errCh := make(chan error, len(tags))
	defer close(errCh)

	for _, tag := range tags {
		wg.Add(1)

		go func(tag string) {
			if _, err := runCommand("docker", []string{"push", tag}); err != nil {
				errCh <- err
			}
			wg.Done()
		}(tag)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
	}

	return nil
}

// buildPublicDockerImage: build a docker image and optionally push it into the public repository
func buildPublicDockerImage(builderConfig *BuilderConfig, projectConfig *ProjectConfig, buildConfig *BuildConfig) error {
	shortGitRef, err := getGitShortRef(projectConfig)
	if err != nil {
		return err
	}

	tags := []string{
		fmt.Sprintf("public/%s:%s", projectConfig.Name, shortGitRef),
		fmt.Sprintf("public/%s:latest", projectConfig.Name),
		fmt.Sprintf("%s/%s:%s", builderConfig.PublicRegistry, projectConfig.Name, shortGitRef),
	}
	if buildConfig.Latest {
		tags = append(tags, fmt.Sprintf("%s/%s:latest", builderConfig.PublicRegistry, projectConfig.Name))
	}

	if err := buildDockerImage(builderConfig, projectConfig, buildConfig, tags); err != nil {
		return err
	}

	if !projectConfig.Push {
		return nil
	}

	return pushDockerImages(tags[2:])
}

// buildPrivateDockerImage: build and (optionally) push a docker image to the private registry
func buildPrivateDockerImage(builderConfig *BuilderConfig, projectConfig *ProjectConfig, buildConfig *BuildConfig) error {
	shortGitRef, err := getGitShortRef(projectConfig)
	if err != nil {
		return err
	}

	tags := []string{
		fmt.Sprintf("private/%s:%s", projectConfig.Name, shortGitRef),
		fmt.Sprintf("private/%s:latest", projectConfig.Name),
		fmt.Sprintf("%s/%s:%s", builderConfig.PrivateRegistry, projectConfig.Name, shortGitRef),
	}

	if buildConfig.Latest {
		tags = append(tags, fmt.Sprintf("%s/%s:latest", builderConfig.PrivateRegistry, projectConfig.Name))
	}

	if err := buildDockerImage(builderConfig, projectConfig, buildConfig, tags); err != nil {
		return err
	}

	if !projectConfig.Push {
		return nil
	}

	return pushDockerImages(tags[2:])
}

// buildLocalDockerImage: build a local docker image
func buildLocalDockerImage(builderConfig *BuilderConfig, projectConfig *ProjectConfig, buildConfig *BuildConfig) error {
	tags := []string{
		fmt.Sprintf("local/%s:latest", projectConfig.Name),
	}

	if err := buildDockerImage(builderConfig, projectConfig, buildConfig, tags); err != nil {
		return err
	}
	return nil
}
