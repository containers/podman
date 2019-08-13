FROM quay.io/libpod/imgts:latest

RUN yum -y update && \
    yum clean all

COPY /contrib/imgprune/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 755 /usr/local/bin/entrypoint.sh
