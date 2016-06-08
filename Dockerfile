FROM ubuntu-debootstrap:trusty
MAINTAINER George Kutsurua <g.kutsurua@gmail.com>

RUN apt-get update && apt-get upgrade -y &&\
    apt-get install -y software-properties-common &&\
    add-apt-repository -y ppa:gluster/glusterfs-3.6 &&\
    apt-get update &&\
    apt-get install -y glusterfs-server lvm2 xfsprogs btrfs-tools fuse &&\
    apt-get clean

COPY entrypoint.sh /usr/bin/entrypoint.sh

ENTRYPOINT /usr/bin/entrypoint.sh
CMD ""

VOLUME ["/var/lib/glusterd", "/mnt/brick"]

EXPOSE 111 24007 2049 38465 38466 38467 1110 4045