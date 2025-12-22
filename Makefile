.PHONY: build run test sqlcGenerate appBuild podmanBuild podmanClean podmanRun podmanCleanAll testBuild testRun

ifndef ENV_FILE
$(error ENV_FILE is not set. Please set it to the path of your environment file, e.g., ENV_FILE=secrets/.env)
endif

include ${ENV_FILE}
export

build: sqlcGenerate appBuild podmanBuild

run: build podmanRun

test: testRun

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

testBuild:
	podman compose --profile test build test-container

testRun: testBuild
	podman compose --profile test up test-container --abort-on-container-exit
