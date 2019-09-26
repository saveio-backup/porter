/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-09-26
 */
package common

import (
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/vrischmann/go-metrics-influxdb"
)

const (
	defaultTableName = "porter"
)

type PorterMetric struct {
	MainConnCounter     metrics.Counter
	ProxyCounter        metrics.Counter
	ProxyConnCounter    metrics.Counter
	TransferAmountGauge metrics.Gauge
	RecvRawTimeGauge    metrics.Gauge
	SendRawTimeGauge    metrics.Gauge
}

func InitMetrics() PorterMetric {
	pm := PorterMetric{
		MainConnCounter:     metrics.NewCounter(),
		ProxyCounter:        metrics.NewCounter(),
		ProxyConnCounter:    metrics.NewCounter(),
		TransferAmountGauge: metrics.NewGauge(),
		SendRawTimeGauge:    metrics.NewGauge(),
		RecvRawTimeGauge:    metrics.NewGauge()}

	metrics.Register("main.conn", pm.MainConnCounter)
	metrics.Register("data.transfer", pm.TransferAmountGauge)
	metrics.Register("proxy.nums", pm.ProxyCounter)
	metrics.Register("proxy.conn.nums", pm.ProxyConnCounter)
	metrics.Register("recv.raw.time", pm.RecvRawTimeGauge)
	metrics.Register("send.raw.time", pm.SendRawTimeGauge)

	go influxdb.InfluxDB(metrics.DefaultRegistry,
		time.Duration(Parameters.InfluxDB.Interval)*time.Second,
		Parameters.InfluxDB.URL,
		Parameters.InfluxDB.DBName,
		defaultTableName,
		Parameters.InfluxDB.UserName,
		Parameters.InfluxDB.Password,
		false)

	return pm
}
