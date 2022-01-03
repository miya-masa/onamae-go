# onamae DDNS go

## Installation

```
go install github.com/miya-masa/onamae-go@latest
```

## Usage

```
# show help
onammae-go -h
```

You can register with systemd. See the unit file example below.

```
[Unit]
Description=onamae

[Service]
Type=simple
Restart=always
ExecStart=/path/to/onamae/bin/onamae-go -h www -d my-domain.com -u <username> -p <password> -daemon
User=<user>
Group=<group>

[Install]
WantedBy=multi-user.target
```

## License

MIT license. See [LICENSE](./LICENSE).
