FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/keda-olm-operator \
    USER_UID=1001 \
    USER_NAME=keda-olm-operator

# install operator binary
COPY bin/manager ${OPERATOR}

COPY bin /usr/local/bin
RUN  /usr/local/bin/user_setup

# install manifest[s]
COPY config /config

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}