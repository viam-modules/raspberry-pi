FROM debian:bullseye

ARG GO_VERSION=1.23.2

RUN --mount=type=cache,target=/var/cache/apt apt-get update
RUN --mount=type=cache,target=/var/cache/apt \
    apt-get install -qqy \
    git build-essential make curl gpg wget sudo nano file procps bash cmake

RUN curl -L https://go.dev/dl/go${GO_VERSION}.linux-$(if [ `dpkg --print-architecture` = arm64 ]; then echo arm64; else echo armv6l; fi).tar.gz | tar -xzv -C /usr/local && \
    update-alternatives --install /usr/bin/go go /usr/local/go/bin/go 10 \
    --slave /usr/bin/gofmt gofmt /usr/local/go/bin/gofmt

# Raspberry Pi repo
RUN curl http://archive.raspberrypi.org/debian/raspberrypi.gpg.key -o /etc/apt/trusted.gpg.d/raspberrypi.asc && echo "deb http://archive.raspberrypi.com/debian $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2) main" > /etc/apt/sources.list.d/raspi.list
RUN --mount=type=cache,target=/var/cache/apt apt-get update

RUN --mount=type=cache,target=/var/cache/apt \
    apt-get install -qqy pigpio

RUN useradd -s /bin/bash -m testbot && echo 'testbot ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Fix for github and new security patches on git
RUN git config --system --add safe.directory '*'

# Verify installations
RUN go version

CMD ["/bin/bash"]
