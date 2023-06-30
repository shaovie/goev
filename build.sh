# sh build.sh ptr
# sh build.sh map
# sh build.sh
GOOS=linux GOARCH=amd64 go build -tags $1 goev
