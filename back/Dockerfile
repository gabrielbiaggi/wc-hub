FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wc-hub ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wc-agent ./cmd/agent
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/terraform-worker ./cmd/terraform-worker

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/wc-hub /app/wc-hub
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/wc-hub"]

FROM gcr.io/distroless/static-debian12:nonroot AS agent
WORKDIR /app
COPY --from=build /out/wc-agent /app/wc-agent
USER nonroot:nonroot
EXPOSE 9105
ENTRYPOINT ["/app/wc-agent"]

FROM hashicorp/terraform:1.15.8 AS terraform-worker
WORKDIR /app
COPY --from=build /out/terraform-worker /app/terraform-worker
USER 65532:65532
EXPOSE 8090
ENTRYPOINT ["/app/terraform-worker"]
