# This is a template Dockerfile

# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Copy the local package files to the container's workspace.
COPY ./ /go/src/dat520.github.io

# Build your application inside the container.
RUN go install dat520.github.io/lab5/PaxosServer

# Run your application when the container starts
ENTRYPOINT ["/go/bin/PaxosServer"]