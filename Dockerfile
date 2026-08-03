FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make nodejs npm

# Copy go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build frontend
WORKDIR /app/web
RUN npm install && npm run build

# Build backend
WORKDIR /app
RUN go build -o vero ./cmd/vero

# Final stage with security tools
FROM alpine:latest

# Install security tools and dependencies
RUN apk add --no-cache \
    curl \
    nmap \
    nmap-scripts \
    python3 \
    py3-pip \
    git \
    bash \
    ca-certificates \
    wget \
    unzip \
    && rm -rf /var/cache/apk/*

# Install nuclei (带 SHA256 校验: 供应链完整性, 修原版 wget 裸下不验)
RUN wget -qO nuclei.zip https://github.com/projectdiscovery/nuclei/releases/download/v3.3.9/nuclei_3.3.9_linux_amd64.zip \
    && echo "dfecedc31364d70b7291b347c74fd4d1d3185d30301c025b7490717d29daf28a  nuclei.zip" | sha256sum -c - \
    && unzip nuclei.zip \
    && mv nuclei /usr/local/bin/ \
    && rm nuclei.zip \
    && nuclei -update-templates

# Install ffuf (带 SHA256 校验)
RUN wget -qO ffuf.tar.gz https://github.com/ffuf/ffuf/releases/download/v2.1.0/ffuf_2.1.0_linux_amd64.tar.gz \
    && echo "fc2c82736c14dcbea4daf3d3cf3878c1c4773008ba45c2bc0fceba7d17b40bb5  ffuf.tar.gz" | sha256sum -c - \
    && tar xzf ffuf.tar.gz \
    && mv ffuf /usr/local/bin/ \
    && rm ffuf.tar.gz

# Install NetExec (nxc)
RUN pip3 install --break-system-packages pipx \
    && export PATH=$PATH:/root/.local/bin \
    && pipx install git+https://github.com/Pennyw0rth/NetExec

# Install impacket (for secretsdump, etc.)
RUN pip3 install --break-system-packages impacket

# Install AWS CLI
RUN pip3 install --break-system-packages awscli

# Install Azure CLI
RUN pip3 install --break-system-packages azure-cli

# Copy binary from builder
COPY --from=builder /app/vero /usr/local/bin/vero

# Create working directory
WORKDIR /workspace

# Expose port
EXPOSE 8000

# Run
ENTRYPOINT ["vero"]
CMD []
