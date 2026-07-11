FROM mcr.microsoft.com/devcontainers/go:2-1.26-trixie

# Install additional OS packages
#
# php-cli + php-curl: needed to run/test sdk/php (lib-forma-php) — the
# SidecarClient uses ext-curl for unix-socket HTTP. Python 3 and Node.js are
# already provided by this base devcontainer image (via its features), so
# sdk/python and sdk/typescript need no extra packages here.
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends sqlite3 php-cli php-curl composer

# [Optional] Uncomment the next lines to use go get to install anything else you need
# USER vscode
# RUN go get -x <your-dependency-or-tool>
# USER root

# [Optional] Uncomment this line to install global node packages.
# RUN su vscode -c "source /usr/local/share/nvm/nvm.sh && npm install -g <your-package-here>" 2>&1
