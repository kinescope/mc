proto:
	@echo "Make proto"

	@protoc -I=. -I=proto -I=${GOPATH}/src \
		--go_out=${GOPATH}/src             \
	proto/cache/*.proto

.PHONY:  proto
