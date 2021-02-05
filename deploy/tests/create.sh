#!/bin/sh

command -v kind >/dev/null 2>&1 || { echo >&2 "Kind not installed.  Aborting."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo >&2 "Kubectl not installed.  Aborting."; exit 1; }
DIR=$(dirname "$0")

if [ -n "${CI_ENV}" ]; then
  echo "cluster was already created by $CI_ENV CI"
else
  kind delete cluster --name dev
  kind create cluster --name dev --config $DIR/kind-config.yaml
fi

if [ -n "${K8S_IC_STABLE}" ]; then
  echo "using last stable image for ingress controller"
  docker pull haproxytech/kubernetes-ingress:latest
  kind --name=dev load docker-image haproxytech/kubernetes-ingress:latest
else
  echo "building image for ingress controller"
  docker build -t haproxytech/kubernetes-ingress -f build/Dockerfile .
  kind --name=dev load docker-image haproxytech/kubernetes-ingress:latest
fi

echo "deploying Ingress Controller ..."
kubectl apply -f $DIR/config/0.namespace.yaml
kubectl apply -f $DIR/config/1.default-backend.yaml
kubectl apply -f $DIR/config/2.rbac.yaml
kubectl apply -f $DIR/config/3.configmap.yaml
kubectl apply -f $DIR/config/4.ingress-controller.yaml
kubectl wait --for=condition=ready --timeout=320s pod -l run=haproxy-ingress -n haproxy-controller
