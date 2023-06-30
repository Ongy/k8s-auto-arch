FROM docker.io/golang:1.19.2-alpine3.16
RUN apk --no-cache add git pkgconfig build-base tzdata alpine-conf
RUN mkdir -p /go/src/github.com/ongy/k8s-auto-arch
ADD . /go/src/github.com/ongy/k8s-auto-arch
WORKDIR /go/src/github.com/ongy/k8s-auto-arch
RUN go install \
    -ldflags="-X main.gitDescribe=$(git -C /go/src/github.com/ongy/k8s-auto-arch/ describe --always --long --dirty)" 

FROM alpine:3.16
WORKDIR /root/
RUN apk --no-cache add tzdata alpine-conf && setup-timezone -z Europe/Berlin
COPY --from=0 /go/bin/mutating-webhook /
CMD ["/mutating-webhook"]
