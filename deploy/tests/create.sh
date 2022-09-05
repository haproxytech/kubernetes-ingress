#!/bin/sh
set -e
clustername=${1:-dev}
command -v kind >/dev/null 2>&1 || { echo >&2 "Kind not installed.  Aborting."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo >&2 "Kubectl not installed.  Aborting."; exit 1; }
DIR=$(dirname "$0")

if [ -n "${CI_ENV}" ]; then
  echo "cluster was already created by $CI_ENV CI"

  echo "building image for ingress controller"
  docker build --build-arg TARGETPLATFORM="linux/amd64" -t haproxytech/kubernetes-ingress -f build/Dockerfile .

  echo "loading image of ingress controller in kind"
  kind load docker-image haproxytech/kubernetes-ingress:latest  --name=$clustername
elif [ -n "${K8S_IC_STABLE}" ]; then
  kind delete cluster --name $clustername
  kind create cluster --name $clustername --config $DIR/kind-config.yaml

  echo "using last stable image for ingress controller"
  docker pull haproxytech/kubernetes-ingress:latest
  kind load docker-image haproxytech/kubernetes-ingress:latest  --name=$clustername
else
  kind delete cluster --name $clustername
  kind create cluster --name $clustername --config $DIR/kind-config.yaml

  echo "building image for ingress controller"
  docker build --build-arg TARGETPLATFORM="linux/amd64" -t haproxytech/kubernetes-ingress -f build/Dockerfile .

  echo "loading image of ingress controller in kind"
  kind load docker-image haproxytech/kubernetes-ingress:latest  --name=$clustername
fi

docker build --build-arg TARGETPLATFORM="linux/amd64" -t haproxytech/http-echo -f deploy/tests/images/http-echo/Dockerfile deploy/tests/images/http-echo
echo "loading image http-echo in kind"
kind load docker-image haproxytech/http-echo:latest  --name=$clustername

echo "deploying Ingress Controller ..."
kubectl apply -f $DIR/config/0.namespace.yaml
kubectl apply -f $DIR/config/1.rbac.yaml
kubectl apply -f $DIR/config/2.configmap.yaml
kubectl apply -f $DIR/config/3.ingress-controller.yaml

echo "wait --for=condition=ready ..."
COUNTER=0
while [  $COUNTER -lt 150 ]; do
    sleep 2
    kubectl get pods -n haproxy-controller --no-headers --selector=run=haproxy-ingress | awk '{print "haproxy-controller/haproxy-kubernetes-ingress " $3 " " $5}'
    result=$(kubectl get pods -n haproxy-controller  --no-headers --selector=run=haproxy-ingress | awk '{print $3}')
    if [ "$result" = "Running" ]; then
      COUNTER=151
    else
      COUNTER=`expr $COUNTER + 1`
    fi
done

kubectl wait --for=condition=ready --timeout=10s pod -l run=haproxy-ingress -n haproxy-controller
