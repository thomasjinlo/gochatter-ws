FROM golang:1.22.3-alpine3.20

WORKDIR /code

# Copy the local package files to the container's workspace
COPY . .

# Download and install any required dependencies
RUN ["go", "install", "./cmd/gochatter-ws"]

# Command to run the executable
CMD ["gochatter-ws"]
