FROM fedora:31

# Install prerequisite
RUN dnf -y upgrade && \
    dnf -y groupinstall "Development Tools" && \
    dnf -y install wget

# Install docker client
RUN wget https://download.docker.com/linux/fedora/31/x86_64/stable/Packages/docker-ce-cli-19.03.9-3.fc31.x86_64.rpm && \
    dnf -y install docker-ce-cli-19.03.9-3.fc31.x86_64.rpm

# Install golang
RUN wget https://dl.google.com/go/go1.13.11.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.13.11.linux-amd64.tar.gz

# Install kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin/kubectl

# Install node
RUN curl -sL https://rpm.nodesource.com/setup_14.x | bash - && \
    dnf install -y nodejs

# Install operator-sdk
RUN RELEASE_VERSION=v0.17.0 && \
    curl -LO https://github.com/operator-framework/operator-sdk/releases/download/${RELEASE_VERSION}/operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu && \
    chmod +x operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu && mkdir -p /usr/local/bin/ && yes | cp operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu /usr/local/bin/operator-sdk

# Prepare environment
ENV PATH=${PATH}:/usr/local/go/bin \
    GOROOT=/usr/local/go \
    GOPATH=/go