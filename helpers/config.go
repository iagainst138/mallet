package helpers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"pacer"
	"recorder"
)

var ConfigError = errors.New("config is invalid, refer to logs")

type CleanupFunc func() error

type BaseConfig struct {
	Addr       string `flag:"a" usage:"address to connect to"`
	NumClients int    `flag:"n" default:"500" usage:"number of test clients to use"`

	ShowResponse bool `flag:"R" usage:"show response"`

	DecreaseThresholdPercentage float64       `flag:"decrease" default:"1.0" usage:"acceptable % decrease allowed in throughput"`
	P95Latency                  time.Duration `flag:"p95" default:"1s" usage:"passing p95 latency"`
}

type SecurityConfig struct {
	InsecureSkipVerify bool `flag:"k" usage:"do not verify TLS"`
}

type PerfConfig struct {
	pacer.Config
	recorder.RecorderConfig
	SecurityConfig
	BaseConfig
}

// HttpConfig can be used for testing against HTTP endpoints
type HttpConfig struct {
	PerfConfig

	ParseResponse bool `flag:"parse-response" usage:"parse response"`

	HttpTimeout time.Duration `flag:"http-timeout" default:"5s" usage:"timeout for http calls"` // TODO fix if passing from perf.py

	GeneratorPath string `flag:"g" usage:"path to generator executable"`
}

func GetHttpConfig() HttpConfig {
	return HttpConfig{
		PerfConfig: PerfConfig{
			Config: pacer.Config{
				Duration: time.Minute,
				RampUp:   time.Second * 1,
				EPS:      1000,
			},

			RecorderConfig: recorder.RecorderConfig{
				CreateStatement:        apiCreateStatement,
				SummaryStatement:       summaryStatement,
				InsertLatencyStatement: apiInsertStatement,
				MinQuery:               minQuery,
				MaxQuery:               maxQuery,
				RollupQuery:            apiRollupQuery,
				DeleteQuery:            deleteStatement,
				MinRollupQuery:         minRollupQuery,
				OldRowsQuery:           oldRowsQuery,

				WindowSize: 1 * time.Second,
			},
		},
	}
}

type SqsConfig struct {
	PerfConfig

	Queue       string        `flag:"Q" usage:"sqs queue"`
	AwsRegion   string        `flag:"A" usage:"aws region"`
	SendTimeout time.Duration `flag:"t" usage:"timeout" default:"1s"`

	LocalStackEndpoint string `flag:"L" usage:"localstack endpoint"`
}

func GetSqsConfig() SqsConfig {
	return SqsConfig{
		PerfConfig: PerfConfig{
			Config: pacer.Config{
				Duration: time.Second * 8,
				RampUp:   time.Second * 1,
				EPS:      100,
			},

			RecorderConfig: recorder.RecorderConfig{
				CreateStatement:        apiCreateStatement,
				SummaryStatement:       summaryStatement,
				InsertLatencyStatement: apiInsertStatement,
				MinQuery:               minQuery,
				MaxQuery:               maxQuery,
				RollupQuery:            apiRollupQuery,
				DeleteQuery:            deleteStatement,
				MinRollupQuery:         minRollupQuery,
				OldRowsQuery:           oldRowsQuery,

				WindowSize: 1 * time.Second,
			},
		},
	}
}

func (s SqsConfig) UsesLocalStack() bool {
	return s.LocalStackEndpoint != ""
}

func (s SqsConfig) Validate() bool {
	ok := true

	if s.Queue == "" {
		ok = false
		slog.Error("config.Validate", "msg", "No queue specified")
	}

	if s.AwsRegion == "" {
		ok = false
		slog.Error("config.Validate", "msg", "No aws region specified")
	}

	return ok
}

func Validator(config PerfConfig) recorder.ValidationFunc {
	expectedRowCount := int(config.Duration.Seconds()) / int(config.WindowSize.Seconds())

	errorQuery := "select sum(error) from results;"

	// here we ignore any windows that are part of the ramp up duration and also any that overflow the length of the test slightly
	throughputQuery := fmt.Sprintf(
		"select AVG(count) from (select * from results where started > (select min(started)+%d from results) limit %d - (select count(*) from results where started < (select min(started)+%d from results)));",
		config.WindowSize.Nanoseconds(),
		expectedRowCount,
		config.WindowSize.Nanoseconds(),
	)

	expectedAverage := float64(config.EPS * int64(config.WindowSize.Seconds()))

	return func(db *sql.DB, percentiles map[float64]time.Duration) ([]string, error) {
		var average float64
		var errorCount int64
		reasons := recorder.NewReasons()
		ok := true

		row := db.QueryRow(errorQuery)
		if row.Err() != nil {
			return nil, row.Err()
		}

		if err := row.Scan(&errorCount); err != nil {
			return nil, err
		}

		if errorCount > 0 {
			ok = false
			reasons.Add(fmt.Sprintf("got %d errors", errorCount))
		}

		row = db.QueryRow(throughputQuery)
		if row.Err() != nil {
			return nil, row.Err()
		}
		if err := row.Scan(&average); err != nil {
			return nil, err
		}

		percentageDecrease := ((expectedAverage - average) / expectedAverage) * 100
		if average < expectedAverage && percentageDecrease > config.DecreaseThresholdPercentage {
			ok = false
			reasons.Add(fmt.Sprintf("average throughput was %.2f but expected %.2f (%.2f%% decrease of %.2f%% threshold)", average, expectedAverage, percentageDecrease, config.DecreaseThresholdPercentage))
		}

		p95, exists := percentiles[95]
		if !exists {
			ok = false
			p95 = -1
			return nil, errors.New("no p95 data")
		} else if p95 > config.P95Latency {
			ok = false
			reasons.Add(fmt.Sprintf("p95 of %s > %s", p95, config.P95Latency))
		}

		if !ok {
			// TODO build up a string of what went wrong here
			return reasons.Get(), errors.New("run failed, refer to logs")
		}

		return nil, nil
	}
}
