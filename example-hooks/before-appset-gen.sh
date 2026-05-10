#!/bin/bash

# Exit silently if APPSET_YAML is not set
if [ -z "$APPSET_YAML" ]; then
    exit 0
fi

# Exit silently if the file does not exist
if [ ! -f "$APPSET_YAML" ]; then
    exit 0
fi

# Check if .metadata.name is state2-apps, exit if not
APPSET_NAME=$(yq eval '.metadata.name' "$APPSET_YAML")
if [ "$APPSET_NAME" != "stage3-apps" ]; then
    exit 0
fi

# Use yq to append "-hooks-edit" to .metadata.name
yq eval '.spec.template.metadata.name += "-hooks-edit"' -i "$APPSET_YAML"

