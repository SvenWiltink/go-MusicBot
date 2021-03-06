#!/bin/sh

# [version tag]-[commits since tag]-[commit hash][-dirty]
VERSION=$(git describe --tags --long --dirty)

if [ -z "${VERSION}" ]; then
    echo "Could not detect version tag"
    exit 1
fi

echo "Building version ${VERSION}"

PKG_ROOT=pkg_root

mkdir -p out/packages
mkdir -p ${PKG_ROOT}/usr/local/bin
mkdir -p ${PKG_ROOT}/usr/local/etc/go-Musicbot

cp out/binaries/MusicBot-linux-amd64 \
    ${PKG_ROOT}/usr/local/bin/go-Musicbot

cp dist/config.json.example \
    ${PKG_ROOT}/usr/local/etc/go-Musicbot/config.json

fpm \
	-n go-musicbot \
	-C ${PKG_ROOT} \
	-s dir \
	-t deb \
	-v "${VERSION}" \
	--force \
	--license MIT \
	-m "Sven Wiltink" \
	--url "https://github.com/svenwiltink/go-musicbot" \
	--description "A musicbot for rocketchat or irc" \
	--config-files /usr/local/etc/go-Musicbot \
        -p "out/packages/${VERSION}.deb"

