.PHONY: run test install-local

run:
	./keel

test:
	go test ./...

install-local:
	./scripts/install-local.sh
