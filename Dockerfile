ARG BUILDPLATFORM
ARG TARGETPLATFORM

FROM --platform=$BUILDPLATFORM docker.io/golang:1.20-alpine AS builder
ARG TARGETARCH
RUN apk --no-cache add git
RUN mkdir -p /go/src/github.com/ongy/k8s-auto-arch
ADD . /go/src/github.com/ongy/k8s-auto-arch
WORKDIR /go/src/github.com/ongy/k8s-auto-arch

# Run the testsuite during container creation.
#RUN CGO_ENABLED=0 go test ./...
RUN GOARCH="${TARGETARCH}" CGO_ENABLED=0 go build -o /go/bin/k8s-auto-arch \
    -ldflags="-w -s -X github.com/ongy/k8s-auto-arch/cmd.gitDescribe=$(git -C /go/src/github.com/ongy/k8s-auto-arch/ describe --always --long --dirty)" 


FROM scratch
ARG TARGETARCH
WORKDIR /
COPY --from=builder /go/bin/k8s-auto-arch /
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
CMD ["/k8s-auto-arch"]
