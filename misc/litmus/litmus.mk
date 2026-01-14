LITMUS_ARGS ?= -k http://localhost:3000

litmus-build:
	docker build -t go-webdav-litmus:latest ./misc/litmus


litmus-run:
	docker run --rm -it --network host go-webdav-litmus:latest /usr/local/bin/litmus $(LITMUS_ARGS)