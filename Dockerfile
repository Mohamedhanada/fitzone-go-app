# stage 1 (build image for app porduction)

FROM golang:1.23-alpine AS build 
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download 
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /fitzone .

# stage 2 (create FInal image for production from build image)
FROM alpine:3.21
WORKDIR /app
RUN adduser -D -H appuser && mkdir -p /app/static/uploads && chown -R appuser:appuser /app/static
COPY --from=build /fitzone /app/fitzone 
COPY static ./static
RUN chown -R appuser:appuser /app/static
USER appuser
EXPOSE 4000 
CMD ["/app/fitzone"]
