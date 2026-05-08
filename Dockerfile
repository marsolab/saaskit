FROM alpine:3.21
RUN apk add --no-cache ca-certificates && mkdir -p /app

COPY api /api
RUN chmod +x /cpage

EXPOSE 8080
ENTRYPOINT ["/api"]
