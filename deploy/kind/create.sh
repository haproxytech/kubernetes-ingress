#!/bin/bash

command -v kind >/dev/null 2>&1 || { echo >&2 "Kind not installed.  Aborting."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo >&2 "Kubectl not installed.  Aborting."; exit 1; }
DIR=$(dirname "$0")

kind delete cluster --name dev
kind create cluster --name dev --config $DIR/kind-config.yaml

kubectl cluster-info --context kind-dev

docker build -t go-web-simple:1.1 https://github.com/oktalz/go-web-simple.git#v1.1.5:.
kind --name="dev" load docker-image  go-web-simple:1.1

sed -e 's/#GROUP#/zagreb/g' -e  's/1 #NUMBER#/4/g' $DIR/web/web-rc.yml | kubectl apply -f -
sed 's/#GROUP#/zagreb/g' $DIR/web/web-svc.yml | kubectl apply -f -

sed -e 's/#GROUP#/paris/g' -e  's/1 #NUMBER#/2/g' $DIR/web/web-rc.yml | kubectl apply -f -
sed 's/#GROUP#/paris/g' $DIR/web/web-svc.yml | kubectl apply -f -

sed -e 's/#GROUP#/waltham/g' -e  's/1 #NUMBER#/2/g' $DIR/web/web-rc.yml | kubectl apply -f -
sed 's/#GROUP#/waltham/g' $DIR/web/web-svc.yml | kubectl apply -f -

kubectl apply -f $DIR/config/0.namespace.yaml
kubectl apply -f $DIR/config/1.ingress.yaml
kubectl apply -f $DIR/config/2.default.yaml
kubectl apply -f $DIR/config/3.rbac.yaml
kubectl apply -f $DIR/config/4.configmap.yaml
kubectl apply -f $DIR/config/4.configmap.tcp.yaml
docker pull haproxytech/kubernetes-ingress:latest
kind --name=dev load docker-image haproxytech/kubernetes-ingress:latest
sed 's|TAG/IMAGE|haproxytech/kubernetes-ingress:latest|g' $DIR/config/5.ingress-controller.yaml | kubectl apply -f -

echo "waiting for pods to be up ..."
kubectl wait --for=condition=ready --timeout=120s pod -l name=web-zagreb
kubectl wait --for=condition=ready --timeout=120s pod -l name=web-paris
kubectl wait --for=condition=ready --timeout=120s pod -l name=web-waltham
kubectl wait --for=condition=ready pod -l run=haproxy-ingress -n haproxy-controller
printf  "sleep a bit more to be consistent\n"
sleep 10

printf  "fetch 8 requests from 4 different pods for hr.haproxy...\n"
x=1; while [ $x -le 8 ]; do curl --header "Host: hr.haproxy" 127.0.0.1:30080/gids; x=$(( $x + 1 )); done
printf  "\nfetch 4 requests from 2 different pods for fr.haproxy...\n"
x=1; while [ $x -le 4 ]; do curl --header "Host: fr.haproxy" 127.0.0.1:30080/gids; x=$(( $x + 1 )); done
printf  "\nfetch 2 requests from 2 different pods for tcp service ...\n"
x=1; while [ $x -le 2 ]; do curl 127.0.0.1:32766/gids; x=$(( $x + 1 )); done
printf  "\nsetup done.\n"
