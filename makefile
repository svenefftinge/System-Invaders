all: systemInvaders

systemInvaders: ./src/main/main.go ./src/space/space.go
	GOPATH=`pwd` go build -ldflags="-s -w" -o systemInvaders main
run:
	GOPATH=`pwd` go run  src/main/main.go
clean:
	@rm -f  systemInvaders 
