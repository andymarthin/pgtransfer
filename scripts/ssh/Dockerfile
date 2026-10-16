# Test SSH server for exercising pgtransfer's SSH tunnel support.
#
# Builds a minimal Alpine sshd with a dedicated tunnel test user and TCP
# forwarding enabled, so `pgtransfer` SSH tunnels can be validated end-to-end.
# The tunnel user's name and password can be overridden at build time:
#   docker build --build-arg SSH_USER=alice --build-arg SSH_PASSWORD=s3cret .
FROM alpine:3.20

ARG SSH_USER=sshuser
ARG SSH_PASSWORD=sshpass

RUN apk add --no-cache openssh \
    && ssh-keygen -A \
    && adduser -D -s /bin/sh "${SSH_USER}" \
    && echo "${SSH_USER}:${SSH_PASSWORD}" | chpasswd

# Enable TCP forwarding (required for SSH tunnels) and password auth.
RUN mkdir -p /etc/ssh/sshd_config.d \
    && printf '%s\n' \
        'PermitRootLogin no' \
        'PasswordAuthentication yes' \
        'AllowTcpForwarding yes' \
        'GatewayPorts yes' \
        'PermitTunnel yes' \
        'X11Forwarding no' \
        > /etc/ssh/sshd_config.d/pgtransfer-test.conf \
    && echo 'Include /etc/ssh/sshd_config.d/*.conf' >> /etc/ssh/sshd_config

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D", "-e"]
