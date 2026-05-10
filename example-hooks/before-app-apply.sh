#!/bin/bash

# Exit silently if APP_YAML is not set
if [ -z "$APP_YAML" ]; then
    exit 0
fi

# Exit silently if the file does not exist
if [ ! -f "$APP_YAML" ]; then
    exit 0
fi

# Check if .metadata.name is state2-apps, exit if not
APP_NAME=$(yq eval '.metadata.name' "$APP_YAML")
if [ "$APP_NAME" != "stage2-apps" ]; then
    exit 0
fi

# Use yq to append "-hooks-edit" to .metadata.name
yq eval '.metadata.name += "-hooks-edit"' -i "$APP_YAML"
