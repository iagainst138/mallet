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
	Path         string
	PageTitle    string
	Title        string
	Subtitle     string
	X            []time.Time
	LatencySerie []Series
	EPSSerie     []Series
}

func (o Options) validate() bool {
	ok := true

	if len(o.X) == 0 {
		slog.Error("charting", "msg", "no X values provided")
		ok = false
	}

	if len(o.LatencySerie) == 0 {
		slog.Error("charting", "msg", "no data points provided")
		ok = false
	}

	return ok
}

type Series struct {
	Name   string
	Values []float64
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
			Theme:     types.ThemeChalk,
			Width:     "99%",
			Height:    "800px",
		}),

		charts.WithYAxisOpts(opts.YAxis{
			Name: "Latency",
			Type: "value",
			Min:  0,
		}),
	)

	line.SetXAxis(o.X)

	line.SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{
		Smooth: &falseBool,
	}))

	line.ExtendYAxis(opts.YAxis{
		Name: "EPS",
		Type: "value",
		Min:  0,
	})

	for _, series := range o.LatencySerie {
		line.AddSeries(series.Name, generateLineItems(series))
	}

	for _, series := range o.EPSSerie {
		line.AddSeries(series.Name, generateLineItems(series), charts.WithLineChartOpts(opts.LineChart{YAxisIndex: 1}))
	}

	f, err := os.Create(o.Path)
	if err != nil {
		return err
	}

	return line.Render(f)
}

func WriteApiChart(path string, windowSize time.Duration, db *sql.DB) error {
	timestamps := make([]time.Time, 0, 1000)
	latencies := make([]float64, 0, 1000)
	minLatencies := make([]float64, 0, 1000)
	p95s := make([]float64, 0, 1000)
	errorCounts := make([]float64, 0, 1000)
	counts := make([]float64, 0, 1000)

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
			errorCount float64
			count      float64
		)
		if err := rows.Scan(&started, &latency, &minLatency, &p95, &errorCount, &count); err != nil {
			return err
		}

		timestamps = append(timestamps, time.Unix(0, started))
		latencies = append(latencies, float64(time.Duration(latency).Microseconds())/1000)
		minLatencies = append(minLatencies, float64(time.Duration(minLatency).Microseconds())/1000)
		p95s = append(p95s, float64(time.Duration(p95).Microseconds())/1000)
		errorCounts = append(errorCounts, errorCount)
		counts = append(counts, float64(count)/windowSize.Seconds())
	}

	chartOptions := Options{
		Path:      path,
		Title:     "Results",
		PageTitle: "Results",
		X:         timestamps,

		LatencySerie: []Series{
			{Name: "max latencies (ms)", Values: latencies},
			{Name: "min latencies (ms)", Values: minLatencies},
			{Name: "rolling p95 (ms)", Values: p95s},
		},

		EPSSerie: []Series{
			{Name: "EPS", Values: counts},
			{Name: "errors", Values: errorCounts},
		},
	}

	return Chart(chartOptions)
}
