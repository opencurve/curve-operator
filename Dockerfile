# Build the curve-operator binary
FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
# RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY pkg/ pkg/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -mod vendor -o curve-operator cmd/main.go

## 
# Use debian-9 as base image to package the curve-operator binary
FROM ubuntu:20.04

WORKDIR /

# Copy curve-operator binary
COPY --from=builder /workspace/curve-operator .

# Configure apt data sources.
RUN echo "deb http://mirrors.aliyun.com/ubuntu/ focal main restricted" > /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-updates main restricted" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal universe" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-updates universe" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal multiverse" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-updates multiverse" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-backports main restricted universe multiverse" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-security main restricted" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-security universe" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-security multiverse" >> /etc/apt/sources.list

# Install utility tools
RUN apt-get update -y && \
    apt-get install -y coreutils dnsutils iputils-ping iproute2 telnet curl vim less wget graphviz unzip tcpdump gdb udev gdisk && \
    apt-get clean

# Install Go
RUN wget https://go.dev/dl/go1.13.15.linux-amd64.tar.gz --no-check-certificate && \
    tar -C /usr/local -xzf go1.13.15.linux-amd64.tar.gz && \
    rm go1.13.15.linux-amd64.tar.gz

Env PATH=$PATH:/usr/local/go/bin

USER root:root

ENTRYPOINT ["./curve-operator"]
