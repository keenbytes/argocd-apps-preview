# argocd-app-of-apps-diff-preview
A lightweight Go utility for generating previews and computing diffs for ArgoCD "app-of-apps" configurations, including 
nested Applications and ApplicationSets.

## Expected Features

- 🌳 Handle nested applications and ApplicationSets
- 📋 Generate previews for app-of-apps configurations
- 🔍 Compute diffs between two sets of application manifests
- 🔄 Branch switching support
- ⚡ Lightweight, two single binaries: one for dumping applications and one for generating diff

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

## Installation
To install the tool, simply download the latest release which contains two binaries: `app-of-apps-dump` and `app-of-apps-diff`.

See example below that downloads the latest release and installs it to `/usr/local/bin`.
```
export APP_OF_APPS_DIR=~/bin/app-of-apps
export APP_OF_APPS_VERSION=0.1.0
export APP_OF_APPS_ARCH=linux_amd64

wget \
  https://github.com/mikolajgasior/argocd-app-of-apps-diff-preview/releases/download/v${APP_OF_APPS_VERSION}/argocd-app-of-apps-diff-preview_${APP_OF_APPS_VERSION}_${APP_OF_APPS_ARCH}.zip \
  -O app-of-apps.zip
  
sudo unzip app-of-apps.zip -x 'LICENSE' -d /usr/local/bin
sudo chmod +x /usr/local/bin/app-of-apps*
```

## Running
### app-of-apps-dump
This binary is used to dump all the ArgoCD ApplicationSets and Applications.

Syntax:
```
Usage:
  app-of-apps-dump [flags]

Flags:
  -h, --help                             help for apps
      --hooks string                     Directory for hooks scripts
      --manifests string                 Directory with start manifests
      --output-apps string               Directory to output app manifests
      --replace-repo-url string          Repository URL to replace
      --replace-target-revision string   Target revision to replace
      --secrets string                   Directory with secrets

```

Use `--replace-repo-url` and `--replace-target-revision` to replace the target repository and revision in the manifests.

### app-of-apps-diff
This binary is used to generate a diff between two sets of ApplicationSets and Applications.

Syntax:
```
Usage:
  diff [flags]

Flags:
      --apps-base string     Manifests from base revision
      --apps-target string   Manifests from target revision
  -h, --help                 help for diff
      --output-diff string   Directory to output diff
```

Use `--apps-base` and `--apps-target` to specify the manifests from the base and target revisions.

### Complete Example

```
# Create output directories
rm -rf outputs*
mkdir outputs-{main,example-1,diff}

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
