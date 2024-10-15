FROM debian:bullseye

ARG GO_VERSION=1.23.2

# Backports repo
RUN echo "deb http://deb.debian.org/debian $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)-backports main" > /etc/apt/sources.list.d/backports.list

RUN --mount=type=cache,target=/var/cache/apt apt-get update
RUN --mount=type=cache,target=/var/cache/apt \
    apt-get install -qqy \
    git build-essential make curl gpg wget sudo nano file procps bash

RUN curl -L https://go.dev/dl/go${GO_VERSION}.linux-$(if [ `dpkg --print-architecture` = arm64 ]; then echo arm64; else echo armv6l; fi).tar.gz | tar -xzv -C /usr/local && \
    update-alternatives --install /usr/bin/go go /usr/local/go/bin/go 10 \
    --slave /usr/bin/gofmt gofmt /usr/local/go/bin/gofmt

# Raspberry Pi repo
RUN curl http://archive.raspberrypi.org/debian/raspberrypi.gpg.key -o /etc/apt/trusted.gpg.d/raspberrypi.asc && echo "deb http://archive.raspberrypi.com/debian $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2) main" > /etc/apt/sources.list.d/raspi.list
RUN --mount=type=cache,target=/var/cache/apt apt-get update

RUN --mount=type=cache,target=/var/cache/apt \
    apt-get install -qqy libpigpio-dev libpigpiod-if-dev pigpio

RUN useradd -s /bin/bash -m testbot && echo 'testbot ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Fix for github and new security patches on git
RUN git config --system --add safe.directory '*'

# Install Node.js
ARG NODE_VERSION=20.x
RUN curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /usr/share/keyrings/nodesource.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/nodesource.gpg] https://deb.nodesource.com/node_${NODE_VERSION} nodistro main" > /etc/apt/sources.list.d/nodesource.list
RUN --mount=type=cache,target=/var/cache/apt apt update && apt install -qqy nodejs

# Verify Node.js installation
RUN node --version

CMD ["/bin/bash"]
