# --- Stage 1: Build Stage ---
# Use an official Go image as the base for our build environment.
# Using a specific version ensures a consistent build environment.
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container.
WORKDIR /app

# Copy the go.mod and go.sum files first. This leverages Docker's layer caching.
# If these files don't change, Docker won't re-download dependencies on subsequent builds.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code into the container.
COPY . .

# Build the Go application.
# CGO_ENABLED=0 creates a statically linked binary (no external dependencies).
# -o /app/server creates an output file named 'server' in the /app directory.
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/api

# --- Stage 2: Final Stage ---
# Use a minimal 'scratch' or 'alpine' image for the final production image.
# 'scratch' is an empty image, making it extremely small and secure.
# 'alpine' is a very small Linux distribution if you need a shell or other tools.
FROM alpine:latest AS production

# Set the working directory.
WORKDIR /app

# Copy only the compiled binary from the 'builder' stage.
# We don't need the Go compiler or any of the source code in the final image.
COPY --from=builder /app/server .

# Copy the migrations folder so the production container can run migrations if needed.
COPY ./internal/migrations ./internal/migrations

# (Optional) If you use a .env file for configuration in production, copy it.
# COPY .env .

# Expose the port that the application will run on.
EXPOSE 8080

# The command to run when the container starts.
CMD ["/app/server"]
