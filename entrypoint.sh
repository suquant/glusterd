#!/bin/bash
#
# This script is designed to be run inside the container
#

# fail hard and fast even on pipelines
set -meo pipefail

# set debug based on envvar
[[ $DEBUG ]] && set -x
service rpcbind start

exec glusterd --log-file=- --no-daemon $@
