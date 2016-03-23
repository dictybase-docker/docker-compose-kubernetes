CURRDIR = $(shell pwd)
FOLDER = $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))/kubernetes-v1.2.0
BIN = $(FOLDER)/_output/dockerized/bin/linux/amd64
k8s_containers = $(shell docker ps -a -f "name=k8s_" -q)
export ENABLE_DAEMON=1
check:
	@echo $(FOLDER)
build:
	cd $(FOLDER) && build/run.sh hack/build-go.sh
kube-start:
	cd $(FOLDER) && hack/local-up-cluster.sh -o $(BIN)
kubectl-setup:
	cd $(FOLDER) && cluster/kubectl.sh config set-cluster local --server=http://127.0.0.1:8080 --insecure-skip-tls-verify=true
	cd $(FOLDER) && cluster/kubectl.sh config set-context local --cluster=local
	cd $(FOLDER) && cluster/kubectl.sh config use-context local
etcd-pod:
	$(BIN)/kubectl create -f $(CURRDIR)/etcd/etcd-service.json
	$(BIN)/kubectl create -f $(CURRDIR)/etcd/etcd-pod.json
kube-ui:
	$(BIN)/kubectl create -f $(FOLDER)/cluster/mesos/docker/kube-system-ns.yaml
	$(BIN)/kubectl create -f $(FOLDER)/cluster/addons/dashboard/dashboard-controller.yaml --namespace=kube-system
	$(BIN)/kubectl create -f $(FOLDER)/cluster/addons/dashboard/dashboard-service.yaml --namespace=kube-system
kube-up: kube-start kubectl-setup kube-ui etcd-pod
kube-stop:
	$(BIN)/kubectl delete rc --all
	$(BIN)/kubectl delete rc --all --namespace=kube-system
	$(BIN)/kubectl delete po --all
	$(BIN)/kubectl delete svc --all
	$(BIN)/kubectl delete pvc --all
	$(BIN)/kubectl delete pv --all
	sudo kill $(shell pgrep -o -f kube-apiserver)
	sudo kill $(shell pgrep -o -f kube-proxy)
	sudo kill $(shell pgrep -o -f kube-controller)
	sudo kill $(shell pgrep -o -f kube-scheduler)
	sudo kill $(shell pgrep -o -f kubelet)
	sudo kill $(shell pgrep -o -f etcd)
kube-cleanup:
ifdef k8s_containers
	docker stop $(k8s_containers)
	docker rm -f -v $(k8s_containers)
endif
kube-down: kube-stop kube-cleanup
