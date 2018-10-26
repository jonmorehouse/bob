package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	yaml "gopkg.in/yaml.v2"
)

func copyDir(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	paths, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, path := range paths {
		dstPath := filepath.Join(dst, path.Name())

		srcPath := filepath.Join(src, path.Name())
		srcPath, err = filepath.EvalSymlinks(srcPath)
		if err != nil {
			return err
		}

		srcStat, err := os.Stat(srcPath)
		if err != nil {
			return err
		}

		if srcStat.IsDir() {
			if err := os.Mkdir(dstPath, srcStat.Mode()); err != nil {
				return err
			}

			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		byts, err := ioutil.ReadFile(srcPath)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(dstPath, byts, srcStat.Mode()); err != nil {
			return err
		}
	}

	return err
}

// createBuildDir: create a build directory removing any symlinks so that it's
// docker build compatible
func createBuildDir(projectConfig *ProjectConfig) (string, error) {
	gitShortRef, err := getGitShortRef(projectConfig)
	if err != nil {
		return "", err
	}

	ts := time.Now()
	tempDir := fmt.Sprintf("/tmp/bob--%s--%s--%v", projectConfig.Name, gitShortRef, int(ts.Unix()))

	if err := os.Mkdir(tempDir, 0755); err != nil {
		return "", err
	}

	if err := copyDir(projectConfig.Dir, tempDir); err != nil {
		return "", err
	}
	return tempDir, nil
}

// build: run a specific build for a project
func build(builderConfig *BuilderConfig, projectConfig *ProjectConfig, buildConfig *BuildConfig) error {
	if buildConfig.Kind == "docker-public" {
		return buildPublicDockerImage(builderConfig, projectConfig, buildConfig)
	}

	if buildConfig.Kind == "docker-private" {
		return buildPrivateDockerImage(builderConfig, projectConfig, buildConfig)
	}

	if buildConfig.Kind == "docker-local" {
		return buildLocalDockerImage(builderConfig, projectConfig, buildConfig)
	}

	if buildConfig.Kind == "bundle" {
		return buildBundle(builderConfig, projectConfig, buildConfig)
	}

	if buildConfig.Kind == "oci" {
		return errors.New("OCI_not_supported_yet")
	}

	return errors.New("invalid build kind  " + buildConfig.Kind)

}

func loadProjectConfig(dir string) (*ProjectConfig, error) {
	configFilepath := path.Join(dir, "build.yml")

	reader, err := os.Open(configFilepath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var projectConfig ProjectConfig
	yamlDecoder := yaml.NewDecoder(reader)

	if err := yamlDecoder.Decode(&projectConfig); err != nil {
		return nil, err
	}

	projectConfig.Dir = dir

	gitShortRef, err := getGitShortRef(&projectConfig)
	if err != nil {
		return nil, err
	}
	projectConfig.GitShortRef = gitShortRef

	gitRef, err := getGitRef(&projectConfig)
	if err != nil {
		return nil, err
	}
	projectConfig.GitRef = gitRef

	return &projectConfig, nil
}

func buildProjectDir(dir string, builderConfig *BuilderConfig, kind string, push bool) error {
	// load projectConfig
	projectConfig, err := loadProjectConfig(dir)
	if err != nil {
		return err
	}

	projectConfig.Push = push

	buildDir, err := createBuildDir(projectConfig)
	if err != nil {
		return err
	}

	projectConfig.BuildDir = buildDir
	if err := os.Chdir(projectConfig.BuildDir); err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(projectConfig.Builds))

	for _, buildConfig := range projectConfig.Builds {
		if kind != "all" && kind != buildConfig.Kind {
			log.Printf("skipping build %s ...", buildConfig.Kind)
			continue
		}

		wg.Add(1)

		go func(b BuildConfig) {
			if err := build(builderConfig, projectConfig, &b); err != nil {
				errCh <- err
			}

			wg.Done()
		}(buildConfig)
	}

	wg.Wait()
	close(errCh)

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	default:
		break
	}

	return nil
}
