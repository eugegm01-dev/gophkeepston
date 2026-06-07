gen-proto:
	protoc --go_out=. --go_opt=module=github.com/eugegm01-dev/gophkeepston \
	       --go-grpc_out=. --go-grpc_opt=module=github.com/eugegm01-dev/gophkeepston \
	       -I api/proto api/proto/*.proto

run-migrations:
	docker compose exec -T postgres psql -U gophkeepston -d gophkeepston < migrations/001_init.sql
	docker compose exec -T postgres psql -U gophkeepston -d gophkeepston < migrations/002_entries.sql
	docker compose exec -T postgres psql -U gophkeepston -d gophkeepston < migrations/003_add_salt.sql
	
run-server:
	go run cmd/server/main.go

run-client:
	go run cmd/client/main.go $(ARGS)

run-tui:
	go run cmd/tui/main.go

build:
	go build -ldflags "-X main.buildVersion=$(VERSION) -X main.buildDate=$(shell date +%Y-%m-%d)" -o bin/gophkeeper cmd/client/main.go
