.PHONY: build run sqlcGenerate appBuild podmanBuild podmanClean podmanRun

include secrets/.env
export

build: sqlcGenerate appBuild podmanBuild

run: build podmanRun

sqlcGenerate:
	sqlc generate -f sqlc/sqlc.yaml

appBuild:
	go build -o build/main ./cmd/main.go

podmanBuild:
	podman compose build

podmanRun: podmanBuild
	podman compose up

podmanClean:
	podman compose down

podmanCleanAll:
	podman compose down -v
