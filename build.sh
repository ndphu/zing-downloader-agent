#!/bin/sh
echo "Killing running agent..."
sudo systemctl stop md

echo "Updating latest source"
git pull

echo "Building new agent..."
go build -o  md-agent

echo "Starting agent..."
sudo systemctl start md

