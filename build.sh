# sh build.sh ptr
# sh build.sh map
# sh build.sh
GOOS=linux GOARCH=amd64 go build -tags $1 goev
GOOS=linux GOARCH=amd64 go build -o /dev/null example/techempower.go
GOOS=linux GOARCH=amd64 go build -o /dev/null example/reuseport.go
GOOS=linux GOARCH=amd64 go build -o /dev/null example/download.go
GOOS=linux GOARCH=amd64 go build -o /dev/null example/echo.go
GOOS=linux GOARCH=amd64 go build -o /dev/null example/websocket.go
GOOS=linux GOARCH=amd64 go build -o /dev/null example/connect_pool.go
GOOS=linux GOARCH=amd64 go build -o /dev/null example/async_http.go
GOOS=linux GOARCH=amd64 go test -o /dev/null -c .
GOOS=linux GOARCH=amd64 go vet .
GOOS=linux GOARCH=amd64 golint .
