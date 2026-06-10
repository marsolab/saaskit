FROM alpine:3.24
RUN apk add --no-cache ca-certificates

# The statically linked Go API binary is built by the CI pipeline (see
# `make build-go`) and copied from the build context.
COPY api /api
RUN chmod +x /api

# HTTP (8080) and gRPC (9090) listeners.
EXPOSE 8080 9090
ENTRYPOINT ["/api"]
