FROM docker.io/golang:1.19.2-alpine3.16
RUN apk --no-cache add git
RUN mkdir -p /go/src/github.com/ongy/k8s-auto-arch
ADD . /go/src/github.com/ongy/k8s-auto-arch
WORKDIR /go/src/github.com/ongy/k8s-auto-arch
RUN CGO_ENABLED=0 go install \
    -ldflags="-w -s -X main.gitDescribe=$(git -C /go/src/github.com/ongy/k8s-auto-arch/ describe --always --long --dirty)" 

FROM scratch
WORKDIR /
COPY --from=0 /go/bin/mutating-webhook /
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
CMD ["/mutating-webhook"]
