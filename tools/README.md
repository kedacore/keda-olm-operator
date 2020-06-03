# Tools for KEDA OLM operator

Simple automation tools to help working with KEDA OLM operator.

## change-repo

Script changes the repository name `kedacore` to some predefined value in all the necessary files.
This can be useful, when you want to push to and pull from other `docker.io` repository, other than `kedacore`.
It needs to be run from the project root directory.

### Usage:
```
Changes repository name in necessary files

-h: print help
-p: change repository to predefined value
-k: change repository to `kedacore`
```

## keda-olm-locally

Script simplifies deploying and running KEDA OLM operator locally.
It needs to be run from the project root directory.

### Usage:
```
Runs keda-olm-operator locally

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
keda-locally -e
```