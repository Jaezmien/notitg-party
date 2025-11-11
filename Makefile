BUILD_DIR = "$(shell pwd)/build"

GOARCH = amd64
VERSION = 0.3.0-alpha
COMMIT = $(shell git rev-parse --short HEAD)

LDFLAGS = -ldflags "-X main.BuildVersion=${VERSION} -X main.BuildCommit=${COMMIT}"

all: clean client-linux client-windows server-linux server-windows
linux: clean client-linux server-linux
windows: clean client-windows server-windows

clean:
	rm -rf "${BUILD_DIR}"
	mkdir "${BUILD_DIR}"

client-linux:
	cd client; \
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILD_DIR}/client-linux-${GOARCH} .

client-windows:
	cd client; \
	GOOS=windows GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILD_DIR}/client-windows-${GOARCH}.exe .

server-linux:
	cd server; \
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILD_DIR}/server-linux-${GOARCH} .

server-windows:
	cd server; \
	GOOS=windows GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILD_DIR}/server-windows-${GOARCH}.exe .
