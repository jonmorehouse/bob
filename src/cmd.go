package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// runCommand: run a command, piping all stdin/stdout while also capturing
// stdin for further use
func runCommand(program string, args []string) ([]byte, error) {
	cmd := exec.Command(program, args...)

	buf := bytes.NewBuffer(nil)
	out := io.MultiWriter(buf, os.Stdin)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return []byte(nil), err
	}

	return buf.Bytes(), nil
}

// runGitCommand: run a git command in the project's source directory
func runGitCommand(projectConfig *ProjectConfig, args ...string) (string, error) {
	args = append([]string{"-C", projectConfig.Dir}, args...)
	res, err := runCommand("git", args)
	if err != nil {
		return "", err
	}

	return strings.Trim(string(res), "\n"), nil
}

// getGitRef: return the full git ref of the project
func getGitRef(projectConfig *ProjectConfig) (string, error) {
	return runGitCommand(projectConfig, "rev-parse", "HEAD")
}

// getGitShortRef: return the current ref
func getGitShortRef(projectConfig *ProjectConfig) (string, error) {
	return runGitCommand(projectConfig, "rev-parse", "--short", "HEAD")
}

// getLastCommitEpoch: returns the timestamp of the last commit for a project
func getLastCommitEpoch(projectConfig *ProjectConfig) (int, error) {
	res, err := runGitCommand(projectConfig, "rev-list", "-1", "HEAD")
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(res)
}
