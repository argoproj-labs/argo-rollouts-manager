# Update argo-rollouts-manager to latest release of Argo Rollouts

The Go code and script this in this directory will automatically open a pull request to update the argo-rollouts-manager to the latest official argo-rollouts release:
- Update container image version in 'default.go'
- Update go.mod to point to latest module version
- Update CRDs to latest
- Open Pull Request using 'gh' CLI

## Instructions

### Pre-requisites:
- GitHub CLI (_gh_) installed and on PATH
- Operator-sdk  v1.28.0 installed (as of January 2024), and on PATH
- Go installed an on PATH

### To run the tool

Modify the `init-repo.sh` file, updating the GitHub URL with a fork.

Then run the script:
```bash
./go-run.sh
```