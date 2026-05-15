# Demo

Ensure that required tools are installed.

## Output directories

Three directories must be created to store the following:
* YAML manifests from base branch (`outputs-main` in this example)
* YAML manifests from target (PR) branch (`outputs-example-1`)
* Difference between the two (`outputs-diff`)

````
rm -rf outputs*
mkdir outputs-{main,example-1,diff}
````

## Start manifests

Initial manifests are required to be present in the `manifests` directory.

## Base branch manifests

As a first step, apply manifests from the `manifests` directory and recursively apply all the Application and
ApplicationSet resources, without syncing them. Use `main` (base branch) when this repository is used. Dump all the
manifests to the `outputs-main` directory.

````
app-of-apps-dump --manifests ./manifests --output-apps ./outputs-main/ \
  --replace-repo-url https://github.com/mikolajgasior/argocd-app-of-apps-diff-preview \
  --replace-target-revision main
````

## Target (PR) branch manifests

Run again but this time with the `example-1` branch. Dump all the manifests to the `outputs-example-1` directory.

```
app-of-apps-dump --manifests ./manifests --output-apps ./outputs-example-1/ \
  --replace-repo-url https://github.com/mikolajgasior/argocd-app-of-apps-diff-preview \
  --replace-target-revision example-1 --hooks ./example-hooks
```

## Diff

Run below command to generate diff between the two branches (`main` and `example-1`).

````
app-of-apps-diff --apps-base ./outputs-main/ --apps-target ./outputs-example-1/ --output-diff ./outputs-diff/
````

Check the `diff.md` file in the `outputs-diff` directory. This file can be used as a comment in a PR.