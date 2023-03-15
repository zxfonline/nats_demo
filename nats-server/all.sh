#!/bin/sh

nats-server -c c1js11.conf > /dev/null 2>&1 &
nats-server -c c1js12.conf > /dev/null 2>&1 &
nats-server -c c1js13.conf > /dev/null 2>&1 &

nats-server -c c2js21.conf > /dev/null 2>&1 &
nats-server -c c2js22.conf > /dev/null 2>&1 &
nats-server -c c2js23.conf > /dev/null 2>&1 &