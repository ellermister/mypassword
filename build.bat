@echo off
go version
go build -ldflags "-s -w -H=windowsgui"