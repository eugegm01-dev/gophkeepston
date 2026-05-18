gen-proto:
	protoc --go_out=. --go_opt=module=github.com/eugegm01-dev/gophkeepston \
	       --go-grpc_out=. --go-grpc_opt=module=github.com/eugegm01-dev/gophkeepston \
	       -I api/proto api/proto/*.proto
		   
run-migrations:
	psql "postgres://gophkeepston:secret@localhost:5432/gophkeepston?sslmode=disable" -f migrations/001_init.sql