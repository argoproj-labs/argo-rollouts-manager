# Update argo-rollouts-manager to latest release of Argo Rollouts

The Go code and script this in this directory will automatically open a pull request to update the argo-rollouts-manager to the latest official argo-rollouts release:
- Update container image version in `default.go`
- Update `go.mod` to point to latest module version
- Update CRDs to latest
- Update target Rollouts version in `hack/run-upstream-argo-rollouts-e2e-tests.sh`
- Open Pull Request using 'gh' CLI

## Instructions

### Prerequisites
- GitHub CLI (_gh_) installed and on PATH
- Go installed an on PATH
- [Operator-sdk v1.28.0](https://github.com/operator-framework/operator-sdk/releases/tag/v1.28.0) installed (as of January 2024), and on PATH
- You must have your own fork of the [argo-rollouts-manager](https://github.com/argoproj-labs/argo-rollouts-manager) repository (example: `jgwest/argo-rollouts-manager`)
- Your local SSH key registered (e.g. `~/.ssh/id_rsa.pub`) with GitHub to allow git clone via SSH

### Configure and run the tool

```bash
export GITHUB_FORK_USERNAME="(your username here)"
export GH_TOKEN="(a GitHub personal access token that can clone/push/open PRs against argo-rollouts-manager repo)"
./init-repo.sh
./go-run.sh
```
