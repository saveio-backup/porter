module github.com/saveio/porter

go 1.14

replace (
	github.com/saveio/carrier => ../carrier
	github.com/saveio/themis => ../themis
)

require (
	github.com/ethereum/go-ethereum v1.10.4
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/lucas-clemente/quic-go v0.12.0
	github.com/mattn/go-sqlite3 v1.14.7
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/pkg/errors v0.9.1
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/saveio/carrier v0.0.0-20210519082359-9fc4d908c385
	github.com/saveio/themis v1.0.123
	github.com/shirou/gopsutil v3.21.6+incompatible
	github.com/syndtr/goleveldb v1.0.1-0.20210305035536-64b5b1c73954
	github.com/vrischmann/go-metrics-influxdb v0.1.1
	github.com/xtaci/kcp-go v5.4.20+incompatible
)
