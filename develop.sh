#!/bin/bash

while true; do
  set -e
  kubectl delete deployment haproxy-ingress-demo || :

  while true; do
    set +e
    make
    ret_code=$?
    if [[ $ret_code -ne 0 ]]
    then
      echo -n "press enter to rebuild again when fixed "
      read something;
      continue
    fi
    break
  done
  set -e
  docker build -t localhost:5000/haproxy-ingress-demo . 
  docker push localhost:5000/haproxy-ingress-demo
  kubectl run -i haproxy-ingress-demo --image=localhost:5000/haproxy-ingress-demo &
  fs/haproxy-ingress-controller -v
  set +e
  while true; do 
    echo -n ">>>>>> [r] rebuild, [p] pod, [enter] for log: "
    read something;
    name=$(kubectl get pods -o go-template --template '{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}' --sort-by='.status.containerStatuses[0].restartCount' | grep haproxy-ingress-demo | head -1);
    name2=$(microk8s.kubectl get pods -o go-template --template '{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}' --sort-by='.status.containerStatuses[0].restartCount' | grep haproxy-ingress-demo | head -1);
    case "$something" in
    r)
      break ;;
    p)
      kubectl describe pod $name;;
    m)
      kubectl logs $name2 ;;
    n)
      kubectl describe pod $name2 ;;
    *)
      kubectl logs $name
    esac
  done
done