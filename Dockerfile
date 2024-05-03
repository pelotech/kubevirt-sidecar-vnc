FROM golang:1.22-alpine

WORKDIR /app
ADD . /app

RUN apk add --no-cache git

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /app/main .

FROM quay.io/kubevirt/sidecar-shim:v1.2.0

COPY --from=0 /app/main .
COPY --from=0 --chmod=755 /app/entrypoint.sh .

EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]