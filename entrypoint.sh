#!/bin/bash
#
# This script is designed to be run inside the container
#

# fail hard and fast even on pipelines
set -meo pipefail

service rpcbind start

# Disable rdma
sed -i.save -e "s#,rdma##" /etc/glusterfs/glusterd.vol

exec glusterd --log-file=- --no-daemon $@
