FROM ubuntu-debootstrap:trusty
MAINTAINER George Kutsurua <g.kutsurua@gmail.com>

RUN apt-get update && apt-get upgrade -y &&\
    apt-get install -y software-properties-common &&\
    add-apt-repository -y ppa:gluster/glusterfs-3.6 &&\
    apt-get update &&\
    apt-get install -y glusterfs-server fuse &&\
    apt-get clean

COPY ["entrypoint.sh", "kubernetes-glusterd", "/"]

VOLUME ["/var/lib/glusterd", "/mnt/brick"]

EXPOSE 111 111/udp 24007 24008 38465 38466 38467 2049 \
       49152 49153 49154 49155

ENTRYPOINT /entrypoint.sh
CMD ""