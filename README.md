## Requirements

- [Golang v1.12+](https://golang.org)
- [Docker v18.09+](https://www.docker.com)
- [Docker Compose v1.23.2+](https://github.com/docker/compose)
- gcc (`sudo apt install build-essential` for Ubuntu)
- [go-bindata](https://github.com/kevinburke/go-bindata) - to generate migrations (in development)

## How to generate file with migrations for the development

    $ make gen

## How to run for the development

    $ make start-docker # to start the development containers
    $ make run # to start a web server

## Hot to build and run

Clone the project into some directory:

    $ cd
    $ git clone git@git.sfxdx.ru:sdr/backend.git
    $ cd backend
    $ git fetch && git checkout develop # switch to the develop branch

Create a docker-compose file from the sample:

    $ cp ./deployment/docker-compose.dev.yml ./deployment/docker-compose.yml

Set the required database credentials in the `services.postgres.environment` section.

Create a configuration file from the sample:

    $ cp ./configs/config.dev.yaml ./configs/config.yaml

Remember to provide correct database credentials in `./configs/config.yaml` for the `database` section.

Run the following command to build the executable binary:

    $ make

Start the docker containers and the binary:

    $ make start-docker
    $ make start
