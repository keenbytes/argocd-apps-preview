# argocd-app-of-apps-diff-preview
A lightweight Go utility for generating previews and computing diffs for ArgoCD "app-of-apps" configurations, including 
nested Applications and ApplicationSets.

## Expected Features

- 🌳 Handle nested applications and ApplicationSets
- 📋 Generate previews for app-of-apps configurations
- 🔍 Compute diffs between two sets of application manifests
- 🔄 Branch switching support
- ⚡ Lightweight, two single binaries: one for dumping applications and one for generating diff
- 
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
# Create output directories
rm -rf outputs*
mkdir outputs-{main,example-1,diff}

# Download binaries
export APP_OF_APPS_DIR=~/bin/app-of-apps
export APP_OF_APPS_VERSION=0.1.0
export APP_OF_APPS_ARCH=linux_amd64
APP_OF_APPS_URL=https://github.com/mikolajgasior/argocd-app-of-apps-diff-preview/releases/download/v${APP_OF_APPS_VERSION}/argocd-app-of-apps-diff-preview_${APP_OF_APPS_VERSION}_${APP_OF_APPS_ARCH}.zip \
APP_OF_APPS_ZIP=$APP_OF_APPS_DIR/app-of-apps.zip && \
mkdir -p $APP_OF_APPS_DIR && \
wget $APP_OF_APPS_URL -O $APP_OF_APPS_ZIP && \
unzip $APP_OF_APPS_ZIP -d $APP_OF_APPS_DIR && \
rm $APP_OF_APPS_ZIP && \
chmod +x $APP_OF_APPS_DIR/app-of-apps*

# Add downloaded binaries to PATH
export PATH=$PATH:$APP_OF_APPS_DIR

# Dump manifests with all the ApplicationSets and Applications
app-of-apps-dump --manifests ./manifests --output-apps ./outputs-main/ \
  --replace-repo-url https://github.com/mikolajgasior/argocd-app-of-apps-diff-preview \
  --replace-target-revision main
app-of-apps-dump --manifests ./manifests --output-apps ./outputs-example-1/ \
  --replace-repo-url https://github.com/mikolajgasior/argocd-app-of-apps-diff-preview \
  --replace-target-revision example-1

# Generate preview of the diff
app-of-apps-diff --apps-base ./outputs-main/ --apps-target ./outputs-example-1/ --output-diff ./outputs-diff/
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
