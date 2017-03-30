# Lightweight Hardended Linux Distro
FROM alpine:3.5

# Update and Install OS level packages
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

# Default build arguments
ARG BINLOC=./eventrelay.linux-amd64
ARG BINDEST=/usr/local/bin/eventrelay

# Add Crest user
RUN adduser -D -H soon_

# Expose Port
EXPOSE 35000

# Copy Binary
COPY ${BINLOC} ${BINDEST}

# Volumes
VOLUME ["/etc/sfm/eventrelay", "/var/log/sfm/eventrelay"]

# Set our Application Entrypoint
ENTRYPOINT ["eventrelay"]
