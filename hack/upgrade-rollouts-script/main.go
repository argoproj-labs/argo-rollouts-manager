package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"os/exec"

	"github.com/google/go-github/v58/github"
)

const (
	PRTitle              = "Upgrade to Argo Rollouts "
	argoRolloutsRepoOrg  = "argoproj"
	argoRolloutsRepoName = "argo-rollouts"

	argoprojlabsRepoOrg         = "argoproj-labs"
	argoRolloutsManagerRepoName = "argo-rollouts-manager"

	skipInitialPRCheck = false // default to false

	// if readOnly is true:
	// - PRs will not be opened
	// - Git commits will not be pushed to fork
	// This is roughly equivalent to a dry run
	readOnly = false // default to false
)

func main() {

	pathToGitHubRepo := "argo-rollouts-manager"

	gitHubToken := os.Getenv("GH_TOKEN")
	if gitHubToken == "" {
		exitWithError(fmt.Errorf("missing GH_TOKEN"))
		return
	}

	client := github.NewClient(nil).WithAuthToken(gitHubToken)

	// 1) Check for existing version update PRs on the repo

	if !skipInitialPRCheck {
		prList, _, err := client.PullRequests.List(context.Background(), argoprojlabsRepoOrg, argoRolloutsManagerRepoName, &github.PullRequestListOptions{})
		if err != nil {
			exitWithError(err)
			return
		}
		for _, pr := range prList {
			if strings.HasPrefix(*pr.Title, PRTitle) {
				exitWithError(fmt.Errorf("PR already exists"))
				return
			}
		}
	}

	// 2) Pull the latest releases from rollouts repo

	releases, _, err := client.Repositories.ListReleases(context.Background(), argoRolloutsRepoOrg, argoRolloutsRepoName, &github.ListOptions{})
	if err != nil {
		exitWithError(err)
		return
	}

	var firstProperRelease *github.RepositoryRelease

	for _, release := range releases {

		if strings.Contains(*release.TagName, "rc") {
			continue
		}
		firstProperRelease = release
		break
	}

	if firstProperRelease == nil {
		exitWithError(fmt.Errorf("no release found"))
		return
	}

	newBranchName := "upgrade-to-rollouts-" + *firstProperRelease.TagName

	// 3) Create, commit, and push a new branch
	if err := createNewCommitAndBranch(*firstProperRelease.TagName, "quay.io/argoproj/argo-rollouts", newBranchName, pathToGitHubRepo); err != nil {
		exitWithError(err)
		return
	}

	if !readOnly {

		bodyText := "Update to latest release of Argo Rollouts"

		if firstProperRelease != nil && firstProperRelease.HTMLURL != nil && *firstProperRelease.HTMLURL != "" {
			bodyText += ": " + *firstProperRelease.HTMLURL
		}

		// 4) Create PR if it doesn't exist
		if stdout, stderr, err := runCommandWithWD(pathToGitHubRepo, "gh", "pr", "create",
			"-R", argoprojlabsRepoOrg+"/"+argoRolloutsManagerRepoName,
			"--title", PRTitle+(*firstProperRelease.TagName), "--body", bodyText); err != nil {
			fmt.Println(stdout, stderr)
			exitWithError(err)
			return
		}
	}

}

func createNewCommitAndBranch(latestReleaseVersionTag string, latestReleaseVersionImage, newBranchName, pathToGitRepo string) error {

	commands := [][]string{
		{"git", "fetch", "parent"},
		{"git", "checkout", "main"},
		{"git", "rebase", "parent/main"},
		{"git", "checkout", "-b", newBranchName},
	}

	if err := runCommandListWithWorkDir(pathToGitRepo, commands); err != nil {
		return err
	}

	if err := regenerateControllersDefaultGo(latestReleaseVersionTag, latestReleaseVersionImage, pathToGitRepo); err != nil {
		return err
	}

	if err := regenerateGoMod(latestReleaseVersionTag, pathToGitRepo); err != nil {
		return err
	}

	if err := regenerateArgoRolloutsE2ETestScriptMod(latestReleaseVersionTag, pathToGitRepo); err != nil {
		return err
	}

	rolloutsRepoPath, err := checkoutRolloutsRepo(latestReleaseVersionTag)
	if err != nil {
		return err
	}

	crdPath := filepath.Join(rolloutsRepoPath, "manifests/crds")
	crdYamlDirEntries, err := os.ReadDir(crdPath)
	if err != nil {
		return err
	}

	var crdYAMLs []string
	for _, crdYamlDirEntry := range crdYamlDirEntries {

		if crdYamlDirEntry.Name() == "kustomization.yaml" {
			continue
		}

		if !crdYamlDirEntry.IsDir() {
			crdYAMLs = append(crdYAMLs, crdYamlDirEntry.Name())
		}
	}

	sort.Strings(crdYAMLs)

	// NOTE: If this line fails, check if any new CRDs have been added to Rollouts, and/or if they have changed the filenames.
	// - If so, this will require verifying the changes, then updating this list
	if !reflect.DeepEqual(crdYAMLs, []string{
		"analysis-run-crd.yaml",
		"analysis-template-crd.yaml",
		"cluster-analysis-template-crd.yaml",
		"experiment-crd.yaml",
		"rollout-crd.yaml"}) {
		return fmt.Errorf("unexpected CRDs found: %v", crdYAMLs)
	}

	destinationPath := filepath.Join(pathToGitRepo, "config/crd/bases")
	for _, crdYAML := range crdYAMLs {

		destFile, err := os.Create(filepath.Join(destinationPath, crdYAML))
		if err != nil {
			return fmt.Errorf("unable to create file for '%s': %w", crdYAML, err)
		}
		defer destFile.Close()

		srcFile, err := os.Open(filepath.Join(crdPath, crdYAML))
		if err != nil {
			return fmt.Errorf("unable to open source file for '%s': %w", crdYAML, err)
		}
		defer srcFile.Close()

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			return fmt.Errorf("unable to copy file for '%s': %w", crdYAML, err)
		}

	}

	commands = [][]string{
		{"go", "mod", "tidy"},
		{"make", "generate", "manifests"},
		{"make", "bundle"},
		{"make", "fmt"},
		{"git", "add", "--all"},
		{"git", "commit", "-s", "-m", PRTitle + latestReleaseVersionTag},
	}
	if err := runCommandListWithWorkDir(pathToGitRepo, commands); err != nil {
		return err
	}

	if !readOnly {
		commands = [][]string{
			{"git", "push", "-f", "--set-upstream", "origin", newBranchName},
		}
		if err := runCommandListWithWorkDir(pathToGitRepo, commands); err != nil {
			return err
		}
	}

	return nil

}

func checkoutRolloutsRepo(latestReleaseVersionTag string) (string, error) {

	tmpDir, err := os.MkdirTemp("", "argo-rollouts-src")
	if err != nil {
		return "", err
	}

	if _, _, err := runCommandWithWD(tmpDir, "git", "clone", "https://github.com/argoproj/argo-rollouts"); err != nil {
		return "", err
	}

	newWD := filepath.Join(tmpDir, "argo-rollouts")

	commands := [][]string{
		{"git", "checkout", latestReleaseVersionTag},
	}

	if err := runCommandListWithWorkDir(newWD, commands); err != nil {
		return "", err
	}

	return newWD, nil
}

func runCommandListWithWorkDir(workingDir string, commands [][]string) error {

	for _, command := range commands {

		_, _, err := runCommandWithWD(workingDir, command...)
		if err != nil {
			return err
		}
	}
	return nil
}

func regenerateGoMod(latestReleaseVersionTag string, pathToGitRepo string) error {

	// Format of string to modify:
	//	github.com/argoproj/argo-rollouts v1.6.3

	path := filepath.Join(pathToGitRepo, "go.mod")

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var res string

	for _, line := range strings.Split(string(fileBytes), "\n") {

		if strings.Contains(line, "\tgithub.com/argoproj/argo-rollouts v") {

			res += "\tgithub.com/argoproj/argo-rollouts " + latestReleaseVersionTag + "\n"

		} else {
			res += line + "\n"
		}

	}

	if err := os.WriteFile(path, []byte(res), 0600); err != nil {
		return err
	}

	return nil

}

func regenerateArgoRolloutsE2ETestScriptMod(latestReleaseVersionTag string, pathToGitRepo string) error {

	// Format of string to modify:
	// CURRENT_ROLLOUTS_VERSION=v1.6.4

	path := filepath.Join(pathToGitRepo, "hack/run-upstream-argo-rollouts-e2e-tests.sh")

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var res string

	for _, line := range strings.Split(string(fileBytes), "\n") {

		if strings.Contains(line, "CURRENT_ROLLOUTS_VERSION=") {

			res += "CURRENT_ROLLOUTS_VERSION=" + latestReleaseVersionTag + "\n"

		} else {
			res += line + "\n"
		}

	}

	if err := os.WriteFile(path, []byte(res), 0600); err != nil {
		return err
	}

	return nil

}

func regenerateControllersDefaultGo(latestReleaseVersionTag string, latestReleaseVersionImage, pathToGitRepo string) error {

	// DefaultArgoRolloutsVersion = "sha256:995450a0a7f7843d68e96d1a7f63422fa29b245c58f7b57dd0cf9cad72b8308f" //v1.4.1

	path := filepath.Join(pathToGitRepo, "controllers/default.go")

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var res string

	for _, line := range strings.Split(string(fileBytes), "\n") {

		if strings.Contains(line, "DefaultArgoRolloutsVersion") {

			res += "	DefaultArgoRolloutsVersion = \"" + latestReleaseVersionTag + "\" // " + latestReleaseVersionTag + "\n"

		} else {
			res += line + "\n"
		}

	}

	if err := os.WriteFile(path, []byte(res), 0600); err != nil {
		return err
	}

	return nil

}

func runCommandWithWD(workingDir string, cmdList ...string) (string, string, error) {

	fmt.Println(cmdList)

	cmd := exec.Command(cmdList[0], cmdList[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Dir = workingDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	fmt.Println(stdoutStr, stderrStr)

	return stdoutStr, stderrStr, err

}

func exitWithError(err error) {
	fmt.Println("ERROR:", err)
	os.Exit(1)
}
