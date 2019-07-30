all: bin

OUTDIR=./build
OUTFILE=sdrwallet

bin:
	go build -ldflags "-s -w" -o ${OUTDIR}/${OUTFILE} ./cmd/main.go

gen:
	go generate ./...

start:
	${OUTDIR}/${OUTFILE}

test:
	go test ./...

##
# docker section for development
##

DOCKER_COMPOSE_FILE=./deployment/docker-compose.yml

start-container:
	docker-compose -f ${DOCKER_COMPOSE_FILE} up -d ${CONTAINER}

stop-container:
	docker-compose -f ${DOCKER_COMPOSE_FILE} stop ${CONTAINER} && docker-compose -f ${DOCKER_COMPOSE_FILE} rm --force ${CONTAINER}

start-postgres:
	CONTAINER=postgres $(MAKE) start-container

stop-postgres:
	CONTAINER=postgres $(MAKE) stop-container

start-docker: start-postgres

stop-docker: stop-postgres

##
# running section for development
##

run:
	go run ./cmd/main.go
