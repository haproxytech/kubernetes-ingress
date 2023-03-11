#!/bin/sh
set -e
clustername=${1:-dev}
command -v kind >/dev/null 2>&1 || { echo >&2 "Kind not installed.  Aborting."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo >&2 "Kubectl not installed.  Aborting."; exit 1; }
DIR=$(dirname "$0")

printf %80s |tr " " "="; echo ""
if [ -n "${CI_ENV}" ]; then
  echo "cluster was already created by $CI_ENV CI"

  echo "building image for ingress controller"
  if [ -n "${GITLAB_CI}" ]; then
    echo "haproxytech/kubernetes-ingress image already available from previous stage"
    #docker build --build-arg TARGETPLATFORM="linux/amd64" -t haproxytech/kubernetes-ingress -f build/Dockerfile .
  else
    docker build --build-arg TARGETPLATFORM="linux/amd64" -t haproxytech/kubernetes-ingress -f build/Dockerfile .
  fi

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

printf %80s |tr " " "="; echo ""
if [ -n "${GITLAB_CI}" ]; then
  echo "haproxytech/http-echo:latest pulled from CI registry"
else
  docker build --build-arg TARGETPLATFORM="linux/amd64" -t haproxytech/http-echo -f deploy/tests/images/http-echo/Dockerfile deploy/tests/images/http-echo
fi
echo "loading image http-echo in kind"
kind load docker-image haproxytech/http-echo:latest  --name=$clustername

printf %80s |tr " " "="; echo ""
echo "Install custom resource definitions ..."
kubectl apply -f $DIR/../../crs/definition/backend.yaml
kubectl apply -f $DIR/../../crs/definition/defaults.yaml
kubectl apply -f $DIR/../../crs/definition/global.yaml

if [ "$EXPERIMENTAL_GWAPI" = "1" ]; then
  printf %80s |tr " " "="; echo ""
  echo "Install experimental GWAPI ..."
  kubectl apply -f $DIR/../../deploy/tests/config/experimental/gwapi.experimental.yaml
  printf %80s |tr " " "="; echo ""
  echo "Install GWAPI resources..."
  echo "kubectl wait --for=condition=ready --timeout=5m pod -l name=gateway-api-admission-server -n gateway-system"
  ####################################################
  COUNTER=0
  while [  $COUNTER -lt 150 ]; do
      FAILED_GWAPI=0
      kubectl wait --for=condition=ready --timeout=5m pod -l name=gateway-api-admission-server -n gateway-system || FAILED_GWAPI=1
      if [ "$FAILED_GWAPI" = "1" ]; then
        COUNTER=`expr $COUNTER + 1`
        sleep 1
      else
        COUNTER=151
      fi
  done
  ####################################################
  kubectl wait --for=condition=ready --timeout=5m pod -l name=gateway-api-admission-server -n gateway-system
  printf %80s |tr " " "="; echo ""
  kubectl apply -f $DIR/../../deploy/tests/config/experimental/gwapi-resources.yaml
  kubectl apply -f $DIR/../../deploy/tests/config/experimental/gwapi-echo-app.yaml
fi

printf %80s |tr " " "="; echo ""
echo "deploying Ingress Controller ..."
kubectl apply -f $DIR/config/0.namespace.yaml
kubectl apply -f $DIR/config/1.rbac.yaml
if [ "$EXPERIMENTAL_GWAPI" = "1" ]; then
  printf %80s |tr " " "="; echo ""
  echo "Install GWAPI resources..."
  kubectl apply -f deploy/tests/config/experimental/gwapi-rbac.yaml
  printf %80s |tr " " "="; echo ""
fi
kubectl apply -f $DIR/config/2.configmap.yaml
if [ "$EXPERIMENTAL_GWAPI" = "1" ]; then
  echo "Adding gateway-controller-name to IC config"
  cat deploy/tests/config/3.ingress-controller.yaml | sed 's#ingress.class=haproxy#&\n            - --gateway-controller-name=haproxy.org/gateway-controller#g' | kubectl apply -f -
else
  kubectl apply -f $DIR/config/3.ingress-controller.yaml
fi
kubectl apply -f $DIR//config/echo-app.yaml
printf %80s |tr " " "="; echo ""

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
