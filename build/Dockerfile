FROM registry.cn-hangzhou.aliyuncs.com/aliknative/golang:1.13.10-alpine3.11 as builder

# Copy local code to the container image.
WORKDIR /go/src/github.com/knative-sample/leader
COPY . .

# Build the command inside the container.
# (You may fetch or manage dependencies here,
# either manually or with a tool like "godep".)
RUN export GO111MODULE=off;CGO_ENABLED=0 GOOS=linux go build -o k8s-controller-leader ./main.go

# Use a Docker multi-stage build to create a lean production image.
# https://docs.docker.com/develop/develop-images/multistage-build/#use-multi-stage-builds
FROM registry.cn-hangzhou.aliyuncs.com/aliknative/alpine:3.11.5
RUN mkdir -p /bin/

# Copy the binary to the production image from the builder stage.
COPY --from=builder /go/src/github.com/knative-sample/leader/k8s-controller-leader /bin/k8s-controller-leader

# Run the web service on container startup.
ENTRYPOINT ["/bin/k8s-controller-leader"]
