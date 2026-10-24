FROM golang:1.26-alpine AS build
ARG VERSION=
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X govite/internal/system/interface/http.Version=${VERSION}" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 3000
ENTRYPOINT ["/api"]
