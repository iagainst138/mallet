package charting

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

type ChartFunc func(string, time.Duration, *sql.DB) error

type Options struct {
	Path      string
	PageTitle string
	Title     string
	Subtitle  string
	X         []time.Time
	Serie     []Series
}

func (o Options) validate() bool {
	ok := true

	if len(o.X) == 0 {
		slog.Error("charting", "msg", "no X values provided")
		ok = false
	}

	if len(o.Serie) == 0 {
		slog.Error("charting", "msg", "no data points provided")
		ok = false
	}

	return ok
}

type Series struct {
	Name   string
	Values []int64
}

func generateLineItems(s Series) []opts.LineData {
	items := make([]opts.LineData, len(s.Values))

	for i := 0; i < len(s.Values); i++ {
		items[i] = opts.LineData{
			Value:  s.Values[i],
			Symbol: "none",
		}
	}

	return items
}

func Chart(o Options) error {
	var (
		trueBool  bool = true
		falseBool bool = false
	)

	if !o.validate() {
		return errors.New("invalid chart options provided, refer to logs")
	}

	line := charts.NewLine()

	// set some global options like Title/Legend/ToolTip or anything else
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWesteros}),

		charts.WithTitleOpts(opts.Title{
			Title:    o.Title,
			Subtitle: o.Subtitle,
		}),

		charts.WithTooltipOpts(opts.Tooltip{
			Show:    &trueBool,
			Trigger: "axis",
		}),

		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),

		charts.WithToolboxOpts(opts.Toolbox{
			Show: &trueBool,
			Feature: &opts.ToolBoxFeature{
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show: &trueBool,
				},
			},
		}),

		charts.WithInitializationOpts(opts.Initialization{
			PageTitle: o.PageTitle,
			Theme:     "westeros",
			Width:     "1500px",
			Height:    "800px",
		}),
	)

	line.SetXAxis(o.X)

	for _, series := range o.Serie {
		line.AddSeries(series.Name, generateLineItems(series))
	}

	line.SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{
		Smooth: &falseBool,
	}))

	f, err := os.Create(o.Path)
	if err != nil {
		return err
	}

	return line.Render(f)
}

func WriteApiChart(path string, windowSize time.Duration, db *sql.DB) error {
	timestamps := make([]time.Time, 0, 1000)
	latencies := make([]int64, 0, 1000)
	minLatencies := make([]int64, 0, 1000)
	p95s := make([]int64, 0, 1000)
	errorCounts := make([]int64, 0, 1000)
	counts := make([]int64, 0, 1000)

	rows, err := db.Query("SELECT started,latency,min_latency,p95,error,count FROM results ORDER BY started;")
	if err != nil {
		return err
	}

	for rows.Next() {
		var (
			started    int64
			latency    int64
			minLatency int64
			p95        int64
			errorCount int64
			count      int64
		)
		if err := rows.Scan(&started, &latency, &minLatency, &p95, &errorCount, &count); err != nil {
			return err
		}

		timestamps = append(timestamps, time.Unix(0, started))
		latencies = append(latencies, time.Duration(latency).Milliseconds())
		minLatencies = append(minLatencies, time.Duration(minLatency).Milliseconds())
		p95s = append(p95s, time.Duration(p95).Milliseconds())
		errorCounts = append(errorCounts, errorCount)
		counts = append(counts, int64(float64(count)/windowSize.Seconds()))
	}

	chartOptions := Options{
		Path:      path,
		Title:     "Results",
		Subtitle:  "If values are showing 0 they are either missing or sub 1ms",
		PageTitle: "Results",
		X:         timestamps,

		Serie: []Series{
			{Name: "max latencies (ms)", Values: latencies},
			{Name: "min latencies (ms)", Values: minLatencies},
			{Name: "errors", Values: errorCounts},
			{Name: "rolling p95 (ms)", Values: p95s},
			{Name: "EPS", Values: counts},
		},
	}

	return Chart(chartOptions)
}

func WriteNatsChart(path string, windowSize time.Duration, db *sql.DB) error {
	timestamps := make([]time.Time, 0, 1000)
	latencies := make([]int64, 0, 1000)
	minLatencies := make([]int64, 0, 1000)
	p95s := make([]int64, 0, 1000)
	errorCounts := make([]int64, 0, 1000)
	counts := make([]int64, 0, 1000)

	ackPendings := make([]int64, 0, 1000)
	minAckPendings := make([]int64, 0, 1000)

	redelivereds := make([]int64, 0, 1000)
	minRedelivereds := make([]int64, 0, 1000)

	waitings := make([]int64, 0, 1000)
	minWaitings := make([]int64, 0, 1000)

	pendings := make([]int64, 0, 1000)
	minPendings := make([]int64, 0, 1000)

	rows, err := db.Query("SELECT started,latency,min_latency,p95,error,count,ack_pending,min_ack_pending,redelivered,min_redelivered,waiting,min_waiting,pending,min_pending FROM results ORDER BY started;")
	if err != nil {
		return err
	}

	for rows.Next() {
		var (
			started    int64
			latency    int64
			minLatency int64
			p95        int64
			errorCount int64
			count      int64

			ackPending    int64
			minAckPending int64

			redelivered    int64
			minRedelivered int64

			waiting    int64
			minWaiting int64

			pending    int64
			minPending int64
		)
		if err := rows.Scan(
			&started,
			&latency,
			&minLatency,
			&p95,
			&errorCount,
			&count,

			&ackPending,
			&minAckPending,

			&redelivered,
			&minRedelivered,

			&waiting,
			&minWaiting,

			&pending,
			&minPending,
		); err != nil {
			return err
		}

		timestamps = append(timestamps, time.Unix(0, started))
		latencies = append(latencies, time.Duration(latency).Milliseconds())
		minLatencies = append(minLatencies, time.Duration(minLatency).Milliseconds())
		p95s = append(p95s, time.Duration(p95).Milliseconds())
		errorCounts = append(errorCounts, errorCount)
		counts = append(counts, int64(float64(count)/windowSize.Seconds()))

		ackPendings = append(ackPendings, ackPending)
		minAckPendings = append(minAckPendings, minAckPending)

		redelivereds = append(redelivereds, redelivered)
		minRedelivereds = append(minRedelivereds, minRedelivered)

		waitings = append(waitings, waiting)
		minWaitings = append(minWaitings, minWaiting)

		pendings = append(pendings, pending)
		minPendings = append(minPendings, minPending)
	}

	chartOptions := Options{
		Path:      path,
		Title:     "Results",
		Subtitle:  "If values are showing 0 they are either missing or sub 1ms",
		PageTitle: "Results",
		X:         timestamps,

		Serie: []Series{
			{Name: "max latencies (ms)", Values: latencies},
			{Name: "min latencies (ms)", Values: minLatencies},
			{Name: "errors", Values: errorCounts},
			{Name: "rolling p95 (ms)", Values: p95s},
			{Name: "EPS", Values: counts},

			{Name: "Max Pending Acks", Values: ackPendings},
			{Name: "Min Pending Acks", Values: minAckPendings},

			{Name: "Max Redelivered", Values: redelivereds},
			{Name: "Min Redelivered", Values: minRedelivereds},

			{Name: "Max Waiting", Values: waitings},
			{Name: "Min Waiting", Values: minWaitings},

			{Name: "Max Pending", Values: pendings},
			{Name: "Min Pending", Values: minPendings},
		},
	}

	return Chart(chartOptions)
}
