#!/usr/bin/env bash

# servers from the backend CRD v3
yq -i 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.servers)' crs/definition/ingress.v3.haproxy.org_backends.yaml
# name (required) from the backend CRD v3
yq -i 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.required[] | select(. == "name"))' crs/definition/ingress.v3.haproxy.org_backends.yaml
# server switching rules from the backend CRD v3: server names are dynamic hashes now
yq -i 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.server_switching_rule_list)' crs/definition/ingress.v3.haproxy.org_backends.yaml
