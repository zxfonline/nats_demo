#!/bin/sh

nats-server -c nats-route1.conf > /dev/null 2>&1 &
nats-server -c nats-route2.conf > /dev/null 2>&1 &
nats-server -c nats-route3.conf > /dev/null 2>&1 &