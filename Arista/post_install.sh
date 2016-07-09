#!/usr/bin/bash

if ! id kapacitor >/dev/null 2>&1; then
   useradd --system -U -M kapacitor
fi

KAPACITORDATA_ROOT=/persist/kapacitor

mkdir -p $KAPACITORDATA_ROOT/data
mkdir -p $KAPACITORDATA_ROOT/replay
mkdir -p $KAPACITORDATA_ROOT/tasks
mkdir -p /var/log/kapacitor

chown -R -L kapacitor:kapacitor $KAPACITORDATA_ROOT
chown -R -L kapacitor:kapacitor /var/log/kapacitor

systemctl enable kapacitor
systemctl restart kapacitor

# even though systemctl waits for kapacitor to be started
# the service is not fully up in terms of accepting client
# connections
# TBD: need a more deterministic way
sleep 30

# refresh complete task list
kapacitor delete tasks hostdown
kapacitor delete tasks disconnects
# add each task 
kapacitor define hostdown -tick /etc/kapacitor/ticks/hostdown.tick -type batch -dbrp ServerStats.default
kapacitor define disconnects -tick /etc/kapacitor/ticks/disconnects.tick -type batch -dbrp ServerStats.default
# enable each task
kapacitor enable hostdown
kapacitor enable disconnects
