package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jonmorehouse/safe"
	yaml "gopkg.in/yaml.v2"
)

type BuilderConfig struct {
	// NOTE: it's expected that `docker login` has been run for both of these projects before running builder
	PrivateRegistry string `yaml:"private_registry"`
	PublicRegistry  string `yaml:"public_registry"`

	GCPCredentialsFilepath string `yaml:"gcp_credentials_filepath"`
	ArtifactorGCSPrefix    string `yaml:"artifactor_gcs_prefix"`
	ArtifactorURLPrefix    string `yaml:"artifactor_url_prefix"`

	baseDir  string
	filepath string
}

type BuildConfig struct {
	Kind   string `yaml:"kind"`
	Latest bool   `yaml:"latest"`

	// Docker internal/public specific
	Labels map[string]string `yaml:"labels"`

	// Binary builds
	Dockerfile string   `yaml:"dockerfile"`
	Versons    []string `yaml:"versions"`

	// OCI builds
}

// findParentFile: walk up the current directory, and find the first file named
// one of the input paths
func findParentFile(filenames ...string) (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	defer func() {
		if err := os.Chdir(startDir); err != nil {
			log.Fatal(err)
		}
	}()

	for {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}

		for _, filename := range filenames {
			if stat, err := os.Stat(filename); err == nil {
				return path.Join(cwd, stat.Name()), nil
			}
		}

		if err := os.Chdir("../"); err != nil {
			break
		}
	}

	return "", errors.New("No bob.yml or bob.yml.gpg.asc file found")
}

// loadBobConfig: load bob config by walking upwards from the current directory
// and looking for a `bob.yml` or `bob.yml.gpg.asc` file
func loadBobConfig(dir string) (*BuilderConfig, error) {
	configFilepath, err := findParentFile("bob.yml", "bob.yml.gpg.asc")
	if err != nil {
		return nil, err
	}

	log.Println("found bob config " + configFilepath)

	var byts []byte
	if strings.HasSuffix(configFilepath, "gpg.asc") {
		byts, err = safe.Decrypt(configFilepath)
	} else {
		byts, err = ioutil.ReadFile(configFilepath)
	}

	if err != nil {
		return nil, err
	}

	var builderConfig BuilderConfig
	if err := yaml.Unmarshal(byts, &builderConfig); err != nil {
		return nil, err
	}

	builderConfig.filepath = configFilepath
	builderConfig.baseDir = filepath.Dir(configFilepath)

	return &builderConfig, nil
}

type ProjectConfig struct {
	Dir            string
	ConfigFilepath string
	BuildDir       string

	GitRef      string
	GitShortRef string

	Name string `yaml:"name"`
	Push bool

	Builds []BuildConfig `yaml:"builds"`
}

type ArtifactManifest struct {
	Timestamp int `json:"unix_timestamp"`
}

// fetchManifest: fetch the latest manifest for a project
func fetchManifest(bobConfig *BuilderConfig, projectConfig *ProjectConfig) (*ArtifactManifest, error) {
	url := fmt.Sprintf("%s/%s/latest/manifest.json", strings.TrimSuffix(bobConfig.ArtifactorURLPrefix, "/"), projectConfig.Name)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("manifest not found")
	}

	var manifest ArtifactManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, err
	}

	log.Println(projectConfig.Name, manifest)

	return &manifest, err
}

// findChangedDirs: find all directories that have been changed since their
// latest build
func findChangedDirs(bobConfig *BuilderConfig) ([]string, error) {
	buildableDirs, err := findBuildableDirs(bobConfig)
	if err != nil {
		return []string(nil), err
	}

	buildDirs := make([]string, 0, len(buildableDirs))

	for _, dir := range buildableDirs {
		projectConfig, err := loadProjectConfig(dir)
		if err != nil {
			return nil, err
		}

		manifest, err := fetchManifest(bobConfig, projectConfig)
		if err != nil {
			log.Println(err)
			buildDirs = append(buildDirs, dir)
			continue
		}

		lastCommitTs, err := getLastCommitEpoch(projectConfig)
		if err != nil {
			buildDirs = append(buildDirs, dir)
			continue
		}

		if manifest.Timestamp > lastCommitTs {
			continue
		}

		buildDirs = append(buildDirs, dir)
	}

	return []string{}, nil
}

// findBuildableDirs: find all buildable directories nested in the parent
// directory
func findBuildableDirs(bobConfig *BuilderConfig) ([]string, error) {
	var dirs []string

	var search func(string) error
	search = func(dir string) error {
		fileinfos, err := ioutil.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, fileinfo := range fileinfos {
			buildDir := path.Join(dir, fileinfo.Name())

			if stat, err := os.Stat(buildDir); err != nil || !stat.IsDir() {
				continue
			}

			buildFilepath := path.Join(buildDir, "build.yml")
			if _, err := os.Stat(buildFilepath); err != nil && os.IsNotExist(err) {
				search(buildDir)
				continue
			}

			buildDir, err := filepath.Abs(buildDir)
			if err != nil {
				return err
			}
			dirs = append(dirs, buildDir)
		}

		return nil
	}

	if err := search(bobConfig.baseDir); err != nil {
		return []string(nil), err
	}

	return dirs, nil
}

func main() {
	var push, changed, all bool
	var kind string

	flag.BoolVar(&push, "push", false, "-push whether to push to  registry/server")
	flag.StringVar(&kind, "kind", "all", "-kind specify a specific build kind; defaults to all")
	flag.BoolVar(&all, "all", false, "-all specifies that all bob compatible subdirectories should be built")
	flag.BoolVar(&changed, "changed", false, "-changed build all projects which have changed since their manifest last changed. NOOP for docker-only builds")
	flag.Parse()

	args := flag.Args()
	if !all && len(args) > 1 {
		log.Fatalf("usage: bob -push=True|False -kind=all|bundle|... <dir>")
	}

	if (all || changed) && len(args) > 0 {
		log.Fatalf("can not pass an argument to -changed and -all builds")
	}

	if all && changed {
		log.Fatalf("can not run -all and -changed at the same time")
	}

	builderConfig, err := loadBobConfig(".")
	if err != nil {
		log.Fatal(err)
	}

	var buildPaths []string
	if all {
		buildPaths, err = findBuildableDirs(builderConfig)
		if err != nil {
			log.Fatal(err)
		}
	} else if changed {
		buildPaths, err = findChangedDirs(builderConfig)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(args) == 1 {
		dir, err := filepath.Abs(args[0])
		if err != nil {
			log.Fatal(err)
		}

		buildPaths = []string{dir}
	} else {
		buildFilepath, err := findParentFile("build.yml")
		if err != nil {
			log.Fatal(err)
		}

		buildPaths = []string{filepath.Dir(buildFilepath)}
	}

	for _, buildPath := range buildPaths {
		log.Println("building " + buildPath)
		if err := buildProjectDir(buildPath, builderConfig, kind, push); err != nil {
			log.Fatal(err)
		}
	}
}
