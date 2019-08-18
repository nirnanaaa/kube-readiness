FROM circleci/golang:1.12

USER root

ADD https://storage.googleapis.com/kubernetes-release/release/v1.13.8/bin/linux/amd64/kubectl /usr/local/bin/kubectl

RUN curl -sL# https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.0.0-rc.0/kubebuilder_2.0.0-rc.0_linux_amd64.tar.gz | tar -xz -C /tmp && \
    mkdir -p /usr/local/kubebuilder/ && \
    mv /tmp/kubebuilder_2.0.0-rc.0_linux_amd64/bin /usr/local/kubebuilder/bin && \
    curl -sL# https://github.com/loadimpact/k6/releases/download/v0.25.1/k6-v0.25.1-linux64.tar.gz | tar -xz -C /tmp && \
    mv /tmp/k6-v0.25.1-linux64/k6 /usr/local/bin/k6 && \
    curl -sL# https://github.com/weaveworks/eksctl/releases/download/latest_release/eksctl_Linux_amd64.tar.gz | tar -xz -C /usr/local/bin && \
    curl -sL# "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip" && \
    unzip -qq awscli-bundle.zip && \
    ./awscli-bundle/install -i /usr/local/aws -b /usr/local/bin/aws && \
    rm -f awscli-bundle.zip && \
    chmod +x /usr/local/bin/kubectl /usr/local/bin/k6 /usr/local/bin/eksctl

USER circleci
