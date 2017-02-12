# replayuploader

SC2 replay uploader for https://sc2replaystats.com.

# Motivation

Started this project to have some way to automatically upload my sc2 replays from Linux.
The current code would probably work for Windows as well, but not macOS until supported by `fsnotify/fsnotify` project.

# Usage

Copy repo and run:

```bash
go run cmd/main.go -dir "/absolute/path/to/replay/dir/" -token "mysc2replaytoken" -hash "sc2replayhash"
```
or use `go get`.


