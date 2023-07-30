# sh build.sh ptr
# sh build.sh map
# sh build.sh
GOOS=linux GOARCH=amd64 go build -tags $1 goev
GOOS=linux GOARCH=amd64 go build -o /dev/null example/techempower.go
GOOS=linux GOARCH=amd64 go build -o /dev/null example/reuseport.go
GOOS=linux GOARCH=amd64 go test -o /dev/null -c .
