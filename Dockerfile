#build stage
FROM reg.ji.com/go-builder:1.20.1-alpine3.17 AS builder
ENV GOSUMDB off
ARG COMMIT_REV
ARG BUILD_DATE
WORKDIR /app
COPY . .
RUN go build -o app -ldflags "-X ji.com/go/x/app.BuildDate=${BUILD_DATE} -X ji.com/go/x/app.CommitRev=${COMMIT_REV}"


#final stage
FROM reg.ji.com/alpine-zh:3.9
WORKDIR /app
#ADD manifest/config /app/config/
#ADD resource /app/resource/
#ADD temp /app/temp/

COPY --from=builder /app/app /app/app

ENTRYPOINT ["/app/app"]
