FROM quay.io/libpod/imgts:latest

RUN yum -y update && \
    yum -y install unzip && \
    rpm -V unzip && \
    yum clean all

COPY /contrib/upldrel/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 755 /usr/local/bin/entrypoint.sh
