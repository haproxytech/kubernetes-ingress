#!/usr/bin/env bash

# servers from the backend CRD v3
yq -i 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.servers)' crs/definition/ingress.v3.haproxy.org_backends.yaml
# name (required) from the backend CRD v3
yq -i 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.required[] | select(. == "name"))' crs/definition/ingress.v3.haproxy.org_backends.yaml
