FROM ubuntu:latest AS build

ARG LITMUS_VERSION=0.17

RUN apt-get update -y && apt-get install -y wget tar build-essential libxml2-dev

RUN mkdir /src \
  && cd /src \
  && wget -O litmus.tar.gz https://notroj.github.io/litmus/litmus-${LITMUS_VERSION}.tar.gz \
  && tar -xzf litmus.tar.gz

WORKDIR /src/litmus-${LITMUS_VERSION}

RUN sh ./configure \
  && make \
  && make DESTDIR=/tmp/build install

FROM ubuntu:latest AS runtime

RUN apt-get update -y && apt-get install -y libxml2

COPY --from=build /tmp/build/ /

CMD ["/usr/local/bin/litmus"]



