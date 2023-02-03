# Client Discovery

## Install

```bash
go get -u github.com/duyunis/discovery
```
## Usage

### Broadcast

```go
broadcast := discovery.NewBroadcast(&discovery.Options{Duration: -1, BroadcastDelay: time.Second * 2})
broadcast.StartBroadcast()
```

### Discovery

```go
discover := discovery.NewDiscover(&discovery.Options{Limit: 1})
broadcast, _ := discover.DiscoverBroadcast()
for _, discovered := range broadcast {
    fmt.Println(discovered.Address)
}
```