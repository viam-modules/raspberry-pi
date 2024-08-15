FROM debian:bookworm

ARG GO_VERSION=1.21

# Backports repo
RUN echo "deb http://deb.debian.org/debian $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)-backports main" > /etc/apt/sources.list.d/backports.list

RUN --mount=type=cache,target=/var/cache/apt apt-get update
RUN --mount=type=cache,target=/var/cache/apt \
    apt-get install -qqy \
    golang-${GO_VERSION}-go git make curl gpg wget sudo nano file procps bash

RUN update-alternatives --install /usr/bin/go go /usr/lib/go-${GO_VERSION}/bin/go 10 \
    --slave /usr/bin/gofmt gofmt /usr/lib/go-${GO_VERSION}/bin/gofmt

#Raspberry Pi repo
RUN curl http://archive.raspberrypi.org/debian/raspberrypi.gpg.key -o /etc/apt/trusted.gpg.d/raspberrypi.asc && echo "deb http://archive.raspberrypi.com/debian $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2) main" > /etc/apt/sources.list.d/raspi.list
RUN --mount=type=cache,target=/var/cache/apt apt-get update

RUN --mount=type=cache,target=/var/cache/apt \
    apt-get install -qqy libpigpio-dev libpigpiod-if-dev

RUN useradd -s /bin/bash -m testbot && echo 'testbot ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Fix for github and new security patches on git
RUN git config --system --add safe.directory '*'

CMD ["/bin/bash"]
