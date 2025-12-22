.PHONY: build run sqlcGenerate appBuild podmanBuild podmanClean podmanRun

ifndef ENV_FILE
$(error ENV_FILE is not set. Please set it to the path of your environment file, e.g., ENV_FILE=secrets/.env)
endif

include ${ENV_FILE}
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
