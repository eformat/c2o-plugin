# Stage 1: Backend builder
FROM docker.io/library/golang:1.23 AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY pkg/ pkg/
RUN CGO_ENABLED=0 GOOS=linux go build -o backend ./cmd/backend/

# Stage 2: Runtime
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
RUN microdnf install -y shadow-utils && microdnf clean all
RUN useradd -u 1001 -r -g 0 -s /sbin/nologin appuser
WORKDIR /app
COPY --from=backend-builder /app/backend .
COPY dist/ dist/
RUN chown -R 1001:0 /app && chmod -R g=u /app
USER 1001
EXPOSE 9443
ENTRYPOINT ["/app/backend"]
