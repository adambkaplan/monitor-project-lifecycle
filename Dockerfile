FROM golang:1.9.4 as builder
WORKDIR /src
ADD . /src
RUN make build

FROM openshift/origin-base
COPY --from=builder /src/_output/local/bin/linux/amd64 /usr/bin

LABEL io.openshift.display-name="Template Monitor" \
      io.openshift.description="Monitor the status of OpenShift's template components, with metrics consumable by Prometheus." \
      io.openshift.tags="openshift" \
      maintainer="Adam Kaplan <adam.kaplan@redhat.com>"

USER 1001
EXPOSE 8080

ENTRYPOINT ["/usr/bin/monitor", "--logtostderr"]
