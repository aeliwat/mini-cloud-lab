.PHONY: build install run docker-build docker-run clean

BINARY ?= minicloud
IMAGE  ?= minicloud

build:
	go build -o $(BINARY) .

# Puts `minicloud` on your PATH (~/go/bin)
install:
	go install .

run: build
	./$(BINARY) $(ARGS)

docker-build:
	docker build -t $(IMAGE) .

docker-run:
	docker run --rm $(IMAGE) $(ARGS)

clean:
	rm -f $(BINARY)
