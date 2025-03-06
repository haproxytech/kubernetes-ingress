#!/usr/bin/env bash

yq -i 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.servers)' crs/definition/ingress.v3.haproxy.org_backends.yaml
