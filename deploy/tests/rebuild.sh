#!/bin/sh
set -e

command -v kind >/dev/null 2>&1 || { echo >&2 "Kind not installed.  Aborting."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo >&2 "Kubectl not installed.  Aborting."; exit 1; }
DIR=$(dirname "$0")

echo "delete image of ingress controller"
kubectl delete -f $DIR/config/4.ingress-controller.yaml

echo "building image for ingress controller"
docker build -t haproxytech/kubernetes-ingress -f build/Dockerfile .
kind --name=dev load docker-image haproxytech/kubernetes-ingress:latest

echo "deploying Ingress Controller ..."
kubectl apply -f $DIR/config/4.ingress-controller.yaml

echo "wait --for=condition=ready ..."
COUNTER=0
while [  $COUNTER -lt 150 ]; do
    sleep 2
    kubectl get pods -n haproxy-controller | grep haproxy-ingress | awk '{print "haproxy-controller/haproxy-ingress " $3 " " $5}'
    result=$(kubectl get pods -n haproxy-controller | grep haproxy-ingress | awk '{print $3}')
    if [ "$result" = "Running" ]; then
      COUNTER=151
    else
      COUNTER=`expr $COUNTER + 1`
    fi
done

kubectl wait --for=condition=ready --timeout=10s pod -l run=haproxy-ingress -n haproxy-controller
