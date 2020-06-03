#!/bin/sh

print_help () {
	echo "Runs keda-olm-operator locally

-h: print help
-n: create keda namespace
-y: install all the necessary yamls
-e: remove all the installed yamls
-u: run operator locally
-i: install keda controller
-r: remove keda controller

Recommended flow:

keda-locally -y
keda-locally -u
keda-locally -i
keda-locally -r
keda-locally -e"
}

create_keda_namespace () {
    kubectl create namespace keda
}

install_yamls () {
    kubectl get namespace keda &>/dev/null || create_keda_namespace
    kubectl apply -f deploy/crds/keda.k8s.io_kedacontrollers_crd.yaml
    kubectl apply -f deploy/resources/crds/keda.k8s.io_scaledobjects_crd.yaml 
    kubectl apply -f deploy/resources/crds/keda.k8s.io_triggerauthentications_crd.yaml 
    kubectl apply -n keda -f deploy/
}

remove_yamls () {
    kubectl delete -n keda -f deploy/
    kubectl delete -f deploy/crds/keda.k8s.io_kedacontrollers_crd.yaml
    kubectl delete -f deploy/resources/crds/keda.k8s.io_scaledobjects_crd.yaml 
    kubectl delete -f deploy/resources/crds/keda.k8s.io_triggerauthentications_crd.yaml 
}

run_locally () {
    operator-sdk run --local --watch-namespace=""
}

install_keda_operator () {
    kubectl apply -n keda -f deploy/crds/keda.k8s.io_v1alpha1_kedacontroller_cr.yaml
}

remove_keda_operator () {
    kubectl delete -n keda -f deploy/crds/keda.k8s.io_v1alpha1_kedacontroller_cr.yaml 
}

while getopts ":hnyeuir" arg; do
  case $arg in
    h)
	    print_help
	    exit 0
	    ;;
    n)
	    create_keda_namespace
	    ;;
    y)
	    install_yamls
	    ;;
    e)
        remove_yamls
        ;;
	u)
        run_locally
		;;
    i)
        install_keda_operator
        ;;
    r)
        remove_keda_operator
        ;;
    *)
        print_help
        exit 0
        ;;
  esac
done
