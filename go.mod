module github.com/desain-gratis/deploy

go 1.24.7

replace github.com/desain-gratis/common => ../common

require (
	github.com/coder/websocket v1.8.14
	github.com/coreos/go-systemd/v22 v22.6.0
	github.com/desain-gratis/common v0.0.0-00010101000000-000000000000
	github.com/julienschmidt/httprouter v1.3.0
	github.com/rs/zerolog v1.34.0
)

require (
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/sys v0.36.0 // indirect
)
