FOLDER = $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))/kubernetes-v1.2.0
check:
	echo $(FOLDER)
build:
	cd $(FOLDER) && build/run.sh hack/build-go.sh
run:
	cd $(FOLDER) && hack/local-up-cluster.sh -o $(FOLDER)/_output/dockerized/bin/linux/amd64

set-kubectl:
	cd $(FOLDER) && cluster/kubectl.sh config set-cluster local --server=http://127.0.0.1:8080 --insecure-skip-tls-verify=true
	cd $(FOLDER) && cluster/kubectl.sh config set-context local --cluster=local
	cd $(FOLDER) && cluster/kubectl.sh config use-context local
	cd $(FOLDER) && cluster/kubectl.sh


