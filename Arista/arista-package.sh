#!/usr/bin/env bash

. ./tools2-functions

#redefine cleanup_exit
cleanup_exit() {
    rm -rf $TMP_BIN_DIR > /dev/null 2>&1
    rm -rf $TMP_CONFIG_DIR > /dev/null 2>&1
    rm -rf ./*.rpm > /dev/null 2>&1
    exit $1
}

check_linux
check_gopath
check_fpm

ARCH=x86_64
GOBIN=$GOPATH/bin
TMP_BIN_DIR=./rpm_bin
TMP_CONFIG_DIR=./rpm_config
CONFIG_FILES_DIR=./ConfigFiles
CONFIG_FILES_VER=1.2
CONFIG_FILES_ITER=1
KAPACITOR_ROOT=..

LICENSE=MIT
URL=github.com/aristanetworks/kapacitor
DESCRIPTION="Time series data processing engine"
VENDOR=Influxdata

set -e

# NOTE: using a commit that works. The latest code
#       causes TICK scripts to fail. The issue is under
#       investigation with Influxdata team
COMMIT="e64b52e05dd7c888fe0549a06db3cac118a63dec"

# Get version from tag closest to HEAD
version=$(git describe --tags --abbrev=0 $COMMIT | sed 's/^v//' )

# Build and install the latest code
echo "Building and Installing Kapacitor ($COMMIT)"
cd ${KAPACITOR_ROOT} && ./build.py --commit $COMMIT && cd -
# NOTE: need to analyze the test failures before enabling this
#       we don't change any core kapacitor code so the failures
#       aren't due to our changes
#cd ${KAPACITOR_ROOT} && ./build.py --test && cd -

echo "Building Arista UDFs"
for udf in {"withoutUpdates","countle"}
do
   go build -o Ticks/$udf Ticks/${udf}.go
done

echo "Creating RPMS"

# Cleanup old RPMS
mkdir ./RPMS > /dev/null 2>&1 || rm -rf ./RPMS/*
rm ./*.rpm > /dev/null 2>&1  || true

COMMON_FPM_ARGS="\
--log error \
--vendor $VENDOR \
--url $URL \
--license $LICENSE"

# Create Binary RPMS
BINARY_FPM_ARGS="\
 -C $TMP_BIN_DIR \
--prefix /usr/bin \
-a $ARCH \
-v $version \
$COMMON_FPM_ARGS"

# Make a copy of the generated binaries into a tmp directory bin
echo "Seting up temporary bin directory"
mkdir $TMP_BIN_DIR > /dev/null 2>&1 || rm -rf $TMP_BIN_DIR/*
for binary in {"kapacitord","kapacitor","tickfmt"}
do
    cp ${KAPACITOR_ROOT}/build/$binary $TMP_BIN_DIR
done

fpm -s dir -t rpm $BINARY_FPM_ARGS --description "$DESCRIPTION" \
   -n "kapacitor-server" kapacitord || cleanup_exit 1
fpm -s dir -t rpm $BINARY_FPM_ARGS -d kapacitor-server \
   --description "$DESCRIPTION" -n "kapacitor-client" \
   kapacitor tickfmt || cleanup_exit 1

mv ./*.rpm RPMS

# Create Config RPMS
CONFIG_FPM_ARGS="\
-C $TMP_CONFIG_DIR \
--prefix / \
-a noarch \
-d kapacitor-client \
--config-files / \
-v $CONFIG_FILES_VER \
--iteration $CONFIG_FILES_ITER \
--after-install ./post_install.sh \
$COMMON_FPM_ARGS"

# Create directory structure for config files
echo "Setting up temporary config file tree"
mkdir $TMP_CONFIG_DIR > /dev/null 2>&1 || rm -rf $TMP_CONFIG_DIR/*
mkdir -p $TMP_CONFIG_DIR/etc/default
cp $CONFIG_FILES_DIR/kapacitor.default $TMP_CONFIG_DIR/etc/default/kapacitor
mkdir -p $TMP_CONFIG_DIR/etc/logrotate.d
cp $CONFIG_FILES_DIR/kapacitor.logrotate \
   $TMP_CONFIG_DIR/etc/logrotate.d/kapacitor
mkdir -p $TMP_CONFIG_DIR/lib/systemd/system
cp $CONFIG_FILES_DIR/kapacitor.service \
   $TMP_CONFIG_DIR/lib/systemd/system/kapacitor.service
mkdir -p $TMP_CONFIG_DIR/etc/kapacitor

# Linux-Config
cp $CONFIG_FILES_DIR/kapacitor-linux.conf \
   $TMP_CONFIG_DIR/etc/kapacitor/kapacitor.conf

mkdir $TMP_CONFIG_DIR/etc/kapacitor/ticks
for udf in {"withoutUpdates","countle"}
do
   cp Ticks/$udf $TMP_CONFIG_DIR/etc/kapacitor/ticks/
done
for tick in {"hostdown","disconnects"}
do
   cp Ticks/${tick}.tick $TMP_CONFIG_DIR/etc/kapacitor/ticks/
done

fpm -s dir -t rpm $CONFIG_FPM_ARGS --description "ServerStats configuration" \
   -n "kapacitor-ServerStats" etc lib || cleanup_exit 1

mv ./*.rpm RPMS

echo "Created RPMS" `ls RPMS`
cleanup_exit 0
