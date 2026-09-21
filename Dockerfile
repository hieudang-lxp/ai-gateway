FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/ backend/
WORKDIR /src/backend
RUN CGO_ENABLED=0 go build -o /gateway ./services/gateway

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /gateway /usr/local/bin/gateway
CMD ["gateway", "api"]
