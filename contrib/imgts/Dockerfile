FROM centos:7

# Only needed for installing build-time dependencies
COPY /contrib/imgts/google-cloud-sdk.repo /etc/yum.repos.d/google-cloud-sdk.repo
RUN yum -y update && \
    yum -y install epel-release && \
    yum -y install google-cloud-sdk && \
    yum clean all

COPY /contrib/imgts/entrypoint.sh /usr/local/bin/entrypoint.sh
ENV GCPJSON="__unknown__" \
    GCPNAME="__unknown__" \
    GCPPROJECT="__unknown__" \
    IMGNAMES="__unknown__" \
    TIMESTAMP="__unknown__" \
    BUILDID="__unknown__" \
    REPOREF="__unknown__"
RUN chmod 755 /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
