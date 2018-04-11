#
# The standard name for this image is adambkaplan/openshift-template-monitor
#
FROM openshift/origin-base

RUN INSTALL_PKGS="openshift-template-monitor" && \
    yum --enablerepo=origin-local-release install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all

LABEL io.openshift.display-name="Template Monitor" \
      io.openshift.description="Monitor the status of OpenShift's template components, with metrics consumable by Prometheus." \
      io.openshift.tags="openshift" \
      maintainer="Adam Kaplan <adam.kaplan@redhat.com>"

USER 1001
EXPOSE 8080

ENTRYPOINT ["/usr/bin/monitor", "--logtostderr"]
