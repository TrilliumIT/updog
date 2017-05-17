FROM golang:1.8 as builder
ENV SRC_ROOT /go/src/github.com/TrilliumIT/updog
WORKDIR ${SRC_ROOT}
RUN go get github.com/Masterminds/glide
RUN go get github.com/jteeuwen/go-bindata/...
ADD glide.* ${SRC_ROOT}/
RUN glide install
ADD . ${SRC_ROOT}/
RUN go generate ./dashboard
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o updog .

FROM alpine:latest
WORKDIR /
COPY --from=builder /go/src/github.com/TrilliumIT/updog/updog .
EXPOSE 8080
CMD ["/updog"]
