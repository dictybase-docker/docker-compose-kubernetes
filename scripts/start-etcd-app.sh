#!/bin/bash

echo "Starting etcd service and pod for applications"

kubectl create --validate=false -f ../etcd/etcd-service.json
kubectl create -f ../etcd/etcd-pod.json
