FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /proxy ./cmd/proxy

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /proxy /proxy
COPY key.pem ./
USER nonroot:nonroot
# EXPOSE 8080
ENTRYPOINT ["/proxy"]
