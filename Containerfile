
FROM registry.fedoraproject.org/fedora-minimal:latest
RUN microdnf install rsync -y && rm -Rf /var/cache/yum
COPY _output/ocp-perf-dash /usr/local/bin/ocp-perf-dash
LABEL maintainer="Raul Sevilla <rsevilla@redhat.com>"
ENTRYPOINT ["/usr/local/bin/ocp-perf-dash"]