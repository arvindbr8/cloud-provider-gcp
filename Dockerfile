FROM --platform=${BUILDPLATFORM} golang:1.26.0 AS builder

# golang envs
ARG TARGETARCH
ARG GOOS=linux
ENV CGO_ENABLED=0
ENV GOARCH=${TARGETARCH}

WORKDIR /go/src/app
COPY go.mod go.sum ./
COPY providers/go.mod providers/go.sum providers/
COPY vendor/ vendor/

COPY cmd/ cmd/
COPY pkg/ pkg/
COPY providers/ providers/

RUN CGO_ENABLED=0 go build -o /go/bin/cloud-controller-manager ./cmd/cloud-controller-manager

FROM gcr.io/gke-release/gke-distroless/go-runner@sha256:3966e3d1257f799098b4abf48204de10111fb03caadd1e854b1d12277157bf1e
COPY --from=builder --chown=root:root /go/bin/cloud-controller-manager /cloud-controller-manager
CMD ["/cloud-controller-manager"]
ENTRYPOINT ["/cloud-controller-manager"]
