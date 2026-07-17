FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wc-hub ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/wc-hub /app/wc-hub
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/wc-hub"]
