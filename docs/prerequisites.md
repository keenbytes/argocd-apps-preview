# Prerequisites

The following tools must be installed:
* kind
* argocd cli
* kubectl
* bash
* git

Kind is used to create a local kubernetes cluster using Docker. kubectl is used to interact with the cluster.

ArgoCD CLI is necessary to connect to the ArgoCD server in the cluster, to perform operations on Application and 
ApplicationSet resources.

Git is used to generate the diff between the applications on the base branch and the target branch.

Bash is used for various operations on the files. These could be coded in Golang but with bash it is easier and more
transparent.

Check below instructions on how to install the tools. Adjust version and destination directory as required.

## Installing kind

```
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.31.0/kind-linux-amd64
chmod +x ./kind
mv ./kind /usr/local/bin/kind
```

## Installing ArgoCD CLI

```
curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/download/v3.0.3/argocd-linux-amd64
chmod +x argocd-linux-amd64
mv ./argocd-linux-amd64 /usr/local/bin/argocd
```
