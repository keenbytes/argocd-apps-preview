# Hooks

## Before App Apply
An Application can be modified before it is applied to the cluster. This is done by using hooks. A bash script can be
executed before the application is applied. 

The script must be called `before-app-apply.sh` and must be placed in a directory which is passed with the `--hooks`
flag. Script is executed with `APP_YAML` environment variable that points to the application yaml file.
See `example-hooks/before-app-apply.sh`.

## Before AppSet Generation
Similarly, before generating Applications from ApplicationSet, a script can be executed. This script must be called
`before-appset-generate.sh` and must be placed in a directory which is passed with the `--hooks` flag. Script is executed
with `APPSET_YAML` environment variable that points to the ApplicationSet yaml file.
See `example-hooks/before-appset-generate.sh`.

