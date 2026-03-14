# syntax=docker/dockerfile:1
FROM osixia/openldap:1.5.0

USER root

# Copy our bootstrap LDIFs into the expected custom directory inside the image
COPY ldif/ /container/service/slapd/assets/config/bootstrap/ldif/custom/

# Ensure ownership matches the openldap user (uid/gid 911 in this image)
RUN chown -R 911:911 /container/service/slapd/assets/config/bootstrap/ldif/custom
