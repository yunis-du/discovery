# Client Discovery

## Install

```bash
go get -u github.com/duyunzhi/discovery
```
## Usage

### Broadcast

```go
broadcast := NewBroadcast(&Options{Duration: -1, BroadcastDelay: time.Second * 2})
broadcast.StartBroadcast()
```

### Discovery

```go
discover := NewDiscover(&Options{Limit: 1})
broadcast, _ := discover.DiscoverBroadcast()
for _, discovered := range broadcast {
    fmt.Println(discovered.Address)
}
```