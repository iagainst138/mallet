package mallet

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/jedib0t/go-pretty/v6/table"
	_ "github.com/mattn/go-sqlite3"

	"mallet/charting"
	"mallet/term"
)

var histogramMax = time.Duration(30 * time.Second).Nanoseconds()

var (
	FlushCount           uint64 = 8000
	ForcedRollupInterval        = time.Second * 30

	ErrTargetsNotMet = errors.New("targets not met")
)

// ValidationFunc should return an error if something has gone wrong and optionally a
// slice of reasons for the error
type ValidationFunc func(*sql.DB, map[float64]time.Duration) ([]string, error)

type TableFunc func(*sql.Rows) (string, error)

type Reasons struct {
	reasons []string
}

func NewReasons() *Reasons {
	return &Reasons{
		reasons: make([]string, 0, 10),
	}
}

func (r *Reasons) Add(s string) {
	slog.Error(s)
	r.reasons = append(r.reasons, s)
}

func (r *Reasons) Get() []string {
	return r.reasons
}

type Recorder struct {
	sync.Mutex

	config RecorderConfig

	histogram *hdrhistogram.Histogram
	db        *sql.DB
	tx        *sql.Tx

	insertCount       atomic.Uint64
	windowSize        time.Duration // TODO fix
	startRollupWindow int64

	createStatement string
	insertStatement *sql.Stmt
	minQuery        *sql.Stmt
	maxQuery        *sql.Stmt
	windowQuery     *sql.Stmt
	rollupQuery     *sql.Stmt
	minRollupQuery  *sql.Stmt
	deleteQuery     *sql.Stmt
	oldRowsQuery    *sql.Stmt

	validationFunc ValidationFunc
	tableFunc      TableFunc
	chartFunc      charting.ChartFunc

	lastRollup time.Time

	int64SlicePool sync.Pool
	recordPool     sync.Pool
}

func NewRecorder(config RecorderConfig, tableFunc TableFunc, chartFunc charting.ChartFunc, validationFunc ValidationFunc) (*Recorder, error) {
	connectionString := fmt.Sprintf("%s?_vacuum=1&_journal_mode=memory", config.getDBPath())

	db, err := sql.Open("sqlite3", connectionString)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(config.CreateStatement); err != nil {
		return nil, err
	}

	r := &Recorder{
		config:         config,
		histogram:      hdrhistogram.New(1, histogramMax, 3),
		db:             db,
		windowSize:     config.WindowSize,
		validationFunc: validationFunc,
		tableFunc:      tableFunc,
		chartFunc:      chartFunc,

		int64SlicePool: sync.Pool{
			New: func() any {
				return make([]int64, 0, 5000)
			},
		},

		recordPool: sync.Pool{
			New: func() any {
				return make([]any, 0, 20)
			},
		},
	}

	if err := r.newTx(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Recorder) RecordLatency(start, finish time.Time, args ...any) {
	duration := finish.Sub(start).Nanoseconds()

	term.Fprintf(os.Stderr, "p95:%s | latency:%s",
		time.Duration(r.histogram.ValueAtPercentile(95)),
		finish.Sub(start),
	)

	r.Lock()
	defer r.Unlock()

	r.histogram.RecordValue(duration)

	records := r.recordPool.Get().([]any)
	records = records[:0]
	records = append(records, start.UnixNano(), duration, duration, r.histogram.ValueAtPercentile(95))
	if len(args) > 0 {
		records = append(records, args...)
	}
	defer r.recordPool.Put(records)

	if _, err := r.insertStatement.Exec(records...); err != nil {
		panic(fmt.Errorf("failed to insert: %w", err))
	}

	if r.insertCount.Add(1) == FlushCount || r.shouldRollup() {
		r.insertCount.Store(0)

		if err := r.newTx(); err != nil {
			// we cannot proceed without the transaction
			panic(fmt.Errorf("failed to create new transaction: %w", err))
		}

		if r.shouldRollup() {
			if err := r.movingRollup(); err != nil {
				log.Println(err)
			}
			r.lastRollup = time.Now()
		}
	}
}

func (r *Recorder) Summary() (string, error) {
	latencyPercentiles := make(map[float64]time.Duration)

	s := "\npercentiles: "
	for _, p := range []float64{25.0, 50.0, 75, 95, 99} {
		duration := time.Duration(r.histogram.ValueAtQuantile(p))
		latencyPercentiles[p] = duration
		s += fmt.Sprintf("%d:%s ", int(p), duration)
	}
	s = strings.TrimSpace(s)

	s += fmt.Sprintf("\n\nmin:%s | max:%s | mean:%s | standard deviation:%s [%f]\n\n",
		time.Duration(r.histogram.Min()),
		time.Duration(r.histogram.Max()),
		time.Duration(r.histogram.Mean()),
		time.Duration(int64(r.histogram.StdDev())),
		r.histogram.StdDev(),
	)

	if err := r.finalRollup(); err != nil {
		return "", fmt.Errorf("finalRollup: %w", err)
	}

	if err := r.tx.Commit(); err != nil {
		return "", err
	}

	rows, err := r.db.Query(r.config.SummaryStatement)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	if r.tableFunc != nil {
		table, err := r.tableFunc(rows)
		if err != nil {
			return "", fmt.Errorf("table error:%w", err)
		}
		s += table
	}

	if r.chartFunc != nil && r.config.ChartPath != "" {
		if err := r.chartFunc(r.config.ChartPath, r.config.WindowSize, r.db); err != nil {
			return s, err
		}
	}

	if r.validationFunc != nil {
		if reasons, err := r.validationFunc(r.db, latencyPercentiles); err != nil {
			for i, reason := range reasons {
				if i == 0 {
					s += "\n\nfailure reasons:\n"
				}
				s += reason + "\n"
			}
			return s, fmt.Errorf("%w:%s", ErrTargetsNotMet, err)
		}
	}

	return s, nil
}

func (r *Recorder) movingRollup() error {
	var windowStart int64
	var windowEnd int64

	if r.startRollupWindow == 0 {
		// this calculates the starting timestamp of our rollup window
		if row := r.minRollupQuery.QueryRow(); row != nil {
			if err := row.Scan(&windowStart); err != nil {
				return fmt.Errorf("minRollupQuery:%w", err)
			}
			r.startRollupWindow = windowStart
		} else {
			return nil
		}
	}

	windowStart = r.startRollupWindow
	windowEnd = windowStart + r.windowSize.Nanoseconds()

	_, err := r.rollup(windowStart, windowEnd)

	// move window forward
	if time.Now().After(time.Unix(0, windowEnd)) {
		r.startRollupWindow = windowEnd
	}

	return err
}

func (r *Recorder) rollup(windowStart, windowEnd int64) (bool, error) {
	rowIds := r.int64SlicePool.Get().([]int64)
	rowIds = rowIds[:0]

	rows, err := r.oldRowsQuery.Query(windowStart, windowEnd)
	if err != nil {
		return false, err
	}

	for rows.Next() {
		var rowId int64
		if err := rows.Scan(&rowId); err != nil {
			return false, err
		}
		rowIds = append(rowIds, rowId)
	}
	defer r.int64SlicePool.Put(rowIds)

	if len(rowIds) == 0 {
		return true, nil
	}

	b, err := json.Marshal(rowIds)
	if err != nil {
		return false, err
	}

	_, err = r.rollupQuery.Exec(windowStart, windowEnd)
	if err != nil {
		return false, fmt.Errorf("rollupQuery[start:%d | end:%d]:%w", windowStart, windowEnd, err)
	}

	// delete old entries
	if _, err := r.deleteQuery.Exec(string(b)); err != nil {
		return false, err
	}

	return false, nil
}

func (r *Recorder) shouldRollup() bool {
	if time.Since(r.lastRollup) >= r.windowSize {
		return true
	} else if time.Since(r.lastRollup) >= ForcedRollupInterval {
		return true
	}

	return false
}

func (r *Recorder) finalRollup() error {
	var windowStart int64

	row := r.minQuery.QueryRow()
	if row == nil {
		return errors.New("failed to get minimum start")
	}
	err := row.Scan(&windowStart)
	if err != nil {
		return err
	}

	done := false
	for !done {
		done, err = r.rollup(windowStart, windowStart+r.windowSize.Nanoseconds())
		if err != nil {
			return err
		}

		if done {
			// here we double check that there are no un-rolled rows
			if row := r.tx.QueryRow("select count(*) from results where rolled_up = 0"); row != nil {
				var count int64
				if err := row.Scan(&count); err != nil {
					return err
				}
				done = count == 0
			}
		}

		windowStart += r.windowSize.Nanoseconds()
	}

	return nil
}

func (r *Recorder) newTx() error {
	var err error

	if r.tx != nil {
		if err := r.tx.Commit(); err != nil {
			return err
		}
	}

	r.tx, err = r.db.Begin()
	if err != nil {
		return err
	}

	r.insertStatement, err = r.tx.Prepare(r.config.InsertLatencyStatement)
	if err != nil {
		return err
	}

	r.deleteQuery, err = r.tx.Prepare(r.config.DeleteQuery)
	if err != nil {
		return err
	}

	r.oldRowsQuery, err = r.tx.Prepare(r.config.OldRowsQuery)
	if err != nil {
		return err
	}

	r.rollupQuery, err = r.tx.Prepare(r.config.RollupQuery)
	if err != nil {
		return err
	}

	r.minRollupQuery, err = r.tx.Prepare(r.config.MinRollupQuery)
	if err != nil {
		return err
	}

	r.windowQuery, err = r.tx.Prepare(r.config.WindowQuery)
	if err != nil {
		return err
	}

	r.minQuery, err = r.tx.Prepare(r.config.MinQuery)
	if err != nil {
		return err
	}

	r.maxQuery, err = r.tx.Prepare(r.config.MaxQuery)
	if err != nil {
		return err
	}

	return nil
}

func ApiTableResults(rows *sql.Rows) (string, error) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"", "Started", "Min", "Max", "P95", "Errors", "Rolled Up", "Count"})

	for rowNum := 1; rows.Next(); rowNum++ {
		var (
			started    int64
			maxLatency int64
			minLatency int64
			p95        int64
			numErrors  int64
			rolledUp   int64
			count      int64
		)
		if err := rows.Scan(&started, &maxLatency, &minLatency, &p95, &numErrors, &rolledUp, &count); err != nil {
			return "", err
		}
		t.AppendRow([]any{
			rowNum,
			time.Unix(0, started).Format("15:04:05.000"),
			time.Duration(minLatency),
			time.Duration(maxLatency),
			time.Duration(p95),
			numErrors,
			rolledUp,
			count,
		})
	}

	return t.Render(), nil
}

func NatsTableResults(rows *sql.Rows) (string, error) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"", "Started",
		"Min Latency", "Max Latency", "P95",
		"ACK Pending", "Min ACK Pending",
		"Redelivered", "Min Redelivered",
		"Waiting", "Min Waiting",
		"Pending", "Min Pending",
		"Errors", "Rolled Up", "Count",
	})

	for rowNum := 1; rows.Next(); rowNum++ {
		var (
			started    int64
			maxLatency int64
			minLatency int64

			maxAckPending int64
			minAckPending int64

			maxRedelivered int64
			minRedelivered int64

			maxWaiting int64
			minWaiting int64

			maxPending int64
			minPending int64

			p95       int64
			numErrors int64
			rolledUp  int64
			count     int64
		)
		if err := rows.Scan(
			&started,
			&maxLatency,
			&minLatency,
			&p95,
			&maxAckPending,
			&minAckPending,
			&maxRedelivered,
			&minRedelivered,
			&maxWaiting,
			&minWaiting,
			&maxPending,
			&minPending,
			&numErrors,
			&rolledUp,
			&count,
		); err != nil {
			return "", err
		}
		t.AppendRow([]any{
			rowNum,
			time.Unix(0, started).Format("15:04:05.000"),
			time.Duration(minLatency),
			time.Duration(maxLatency),
			time.Duration(p95),
			maxAckPending,
			minAckPending,
			maxRedelivered,
			minRedelivered,
			maxWaiting,
			minWaiting,
			maxPending,
			minPending,
			numErrors,
			rolledUp,
			count,
		})
	}

	return t.Render(), nil
}
