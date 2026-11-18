# syntax=docker/dockerfile:1
ARG OPENSEARCH_TAG=3.7.0
FROM opensearchproject/opensearch:${OPENSEARCH_TAG}

USER root

# Install repository-s3 plugin for acceptance tests (S3 snapshot repository)
RUN /usr/share/opensearch/bin/opensearch-plugin install --batch repository-s3 \
    && chown -R opensearch:opensearch /usr/share/opensearch

USER opensearch
