#!/bin/sh
echo "Killing running agent..."
sudo pkill md-agent

echo "Updating latest source"
git pull

echo "Building new agent..."
go build -o  md-agent

echo "Starting agent..."
nohup ./md-agent &

