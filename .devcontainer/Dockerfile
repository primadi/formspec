FROM mcr.microsoft.com/devcontainers/go:2-1.26-trixie

# Install additional OS packages
#
# php-cli + php-curl: needed to run/test sdk/php (lib-forma-php) — the
# SidecarClient uses ext-curl for unix-socket HTTP. Python 3 and Node.js
# are provided by devcontainer.json features (see .devcontainer/devcontainer.json),
# so sdk/python and sdk/typescript need no extra packages here.
# Java (JDK 17 + Maven), .NET SDK, and Ruby are COMMENTED OUT by default to save
# ~1.1 GB disk + JVM RAM. Uncomment in devcontainer.json when working on
# sdk/java, sdk/dotnet, or sdk/ruby, then rebuild the container.
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends sqlite3 php-cli php-curl composer

# Install rtk (Rust Token Killer) — CLI proxy that compresses shell output
# before it reaches LLM agents (Claude Code / Copilot hook).
#   - Installed as the `vscode` user so the binary lands in ~/.local/bin,
#     which is auto-added to PATH by ~/.profile in the base image.
#   - Without RTK_VERSION the installer fetches the latest release (see rtk-ai/rtk releases).
#   - `rtk init -g` registers the global Claude Code hook + RTK.md. That state
#     lives under $HOME, so it must be baked here to survive a container rebuild.
#     RTK_TELEMETRY_DISABLED=1 keeps the step non-interactive.
USER vscode
RUN curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh -o /tmp/rtk-install.sh \
    && sh /tmp/rtk-install.sh \
    && rm -f /tmp/rtk-install.sh \
    && mkdir -p /home/vscode/.claude \
    && RTK_TELEMETRY_DISABLED=1 /home/vscode/.local/bin/rtk init -g
USER root

# [Optional] Uncomment the next lines to use go get to install anything else you need
# USER vscode
# RUN go get -x <your-dependency-or-tool>
# USER root

# [Optional] Uncomment this line to install global node packages.
# RUN su vscode -c "source /usr/local/share/nvm/nvm.sh && npm install -g <your-package-here>" 2>&1
