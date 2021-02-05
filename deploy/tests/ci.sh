#!/bin/bash

docker build -t haproxytech/kubernetes-ingress -f build/Dockerfile .
kind --name=dev load docker-image haproxytech/kubernetes-ingress:latest

echo "deploying Ingress Controller ..."
kubectl apply -f deploy/tests/config/0.namespace.yaml
kubectl apply -f deploy/tests/config/1.default-backend.yaml
kubectl apply -f deploy/tests/config/2.rbac.yaml
kubectl apply -f deploy/tests/config/3.configmap.yaml
kubectl apply -f deploy/tests/config/4.ingress-controller.yaml
kubectl wait --for=condition=ready pod -l run=haproxy-ingress -n haproxy-controller
