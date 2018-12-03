FROM golang:1.11-stretch AS fswatch-builder

WORKDIR /src/fswatch

# Cache dependency source.
COPY fswatch/go.mod fswatch/go.sum ./
RUN go mod download

COPY fswatch/main.go ./
RUN go build -v -mod=readonly


FROM golang:1.11-stretch AS e2e-builder

WORKDIR /src/e2e

# Cache dependency source.
COPY e2e/go.mod e2e/go.sum ./
RUN go mod download

COPY e2e/*.go ./
RUN go test -c -o e2e -mod=readonly


FROM debian:stretch-slim

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get dist-upgrade --no-install-recommends -y \
 && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
   python3 \
   sudo \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* \
 && groupadd testguy \
 && useradd -g testguy -m testguy \
 && groupadd sudoers \
 && usermod -a -G sudoers testguy \
 && echo 'Defaults:%sudoers closefrom_override' >>/etc/sudoers.d/suoders-closefrom \
 && echo '%sudoers ALL = (ALL:ALL) NOPASSWD: ALL' >>/etc/sudoers.d/suoders-nopasswd

COPY --from=fswatch-builder \
  /src/fswatch/fswatch \
  /usr/local/bin/fswatch

COPY --from=e2e-builder \
  /src/e2e/e2e \
  /usr/local/bin/e2e

WORKDIR /home/testguy

USER testguy:testguy

CMD ["/usr/local/bin/e2e", "-test.v"]
