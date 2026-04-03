# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependencies separately from source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin//var/folders/cr/4ch4t4hd0nd1t8tk23km89h80000gn/T/TestScaffoldForce3092793338/002/my-app .

# ---- Final stage ----
FROM gcr.io/distroless/static-debian12
COPY --from=builder /bin//var/folders/cr/4ch4t4hd0nd1t8tk23km89h80000gn/T/TestScaffoldForce3092793338/002/my-app //var/folders/cr/4ch4t4hd0nd1t8tk23km89h80000gn/T/TestScaffoldForce3092793338/002/my-app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["//var/folders/cr/4ch4t4hd0nd1t8tk23km89h80000gn/T/TestScaffoldForce3092793338/002/my-app", "serve"]
