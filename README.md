# argocd-app-of-apps-diff-preview
A lightweight Go utility for generating previews and computing diffs for ArgoCD "app-of-apps" configurations, including 
nested Applications and ApplicationSets.

## Expected Features

- 🌳 Handle nested applications and ApplicationSets
- 📋 Generate previews for app-of-apps configurations
- 🔍 Compute diffs between two sets of application manifests
- 🔄 Branch switching support
- ⚡ Lightweight, single binary distribution

## Motivation

While there are excellent tools available today for previewing ArgoCD changes, most struggle with the complexity of 
"app-of-apps" patterns where applications nest other applications or ApplicationSets.

I designed this tool specifically with **Pull Request workflows** in mind. When reviewing changes that span multiple 
Helm charts and Kustomize overlays, it is often difficult to determine exactly what will be applied to the cluster after
the merge. Existing tools typically stop at the top-level manifest, leaving reviewers blind to the downstream effects of
nested configurations.

This utility bridges that gap by recursively resolving the entire application tree. It provides a clear, accurate diff 
preview of the final manifests that ArgoCD would apply, making code reviews for complex GitOps strategies significantly 
safer and more efficient.

## Prerequisites

The following tools must be installed:
* kind
* argocd cli
* kubectl
* bash
* git

## Running

```
rm -rf outputs*
mkdir outputs-{main,example-1,diff}
cd cmd/apps/ && go build . && cd ../../
cd cmd/diff/ && go build . && cd ../../

./cmd/apps/apps --manifests ./manifests --output-apps ./outputs-main/ \
  --replace-repo-url https://github.com/mikolajgasior/argocd-app-of-apps-diff-preview \
  --replace-target-revision main

./cmd/apps/apps --manifests ./manifests --output-apps ./outputs-example-1/ \
  --replace-repo-url https://github.com/mikolajgasior/argocd-app-of-apps-diff-preview \
  --replace-target-revision example-1

./cmd/diff/diff --apps-base ./outputs-main/ --apps-target ./outputs-example-1/ --output-diff ./outputs-diff/
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
