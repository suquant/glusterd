FROM ubuntu-debootstrap:trusty
MAINTAINER George Kutsurua <g.kutsurua@gmail.com>

RUN apt-get update && apt-get upgrade -y &&\
    apt-get install -y software-properties-common &&\
    add-apt-repository -y ppa:gluster/glusterfs-3.7 &&\
    apt-get update &&\
    apt-get install -y glusterfs-server &&\
    apt-get clean

COPY entrypoint.sh /usr/bin/entrypoint.sh

VOLUME ["/volumes"]

ENTRYPOINT /usr/bin/entrypoint.sh
CMD ""

EXPOSE 111 24007 2049 38465 38466 38467 1110 4045