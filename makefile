generate:
	mkdir -p internal/protos
	protoc -I docs/protos \
		docs/protos/gokit/ping/v1/*.proto \
		--go_out=plugins=grpc:./internal/protos