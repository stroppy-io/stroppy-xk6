package stroppy_xk6

import (
	"errors"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/modules"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-core/pkg/logger"
	"github.com/stroppy-io/stroppy-core/pkg/plugins/driver"
	stroppy "github.com/stroppy-io/stroppy-core/pkg/proto"
)

const (
	pluginLoggerName = "XK6Plugin"
)

type Instance struct {
	vu      modules.VU
	exports *sobek.Object
	logger  *zap.Logger
}

func NewXK6Instance(vu modules.VU, exports *sobek.Object) *Instance {
	lg := logger.NewFromEnv().
		Named(pluginLoggerName).
		WithOptions(zap.AddCallerSkip(1))
	if vu.State() != nil {
		vu.State().Logger = NewZapFieldLogger(lg)
	}

	return &Instance{
		vu:      vu,
		exports: exports,
		logger:  lg,
	}
}

func (x *Instance) New() *Instance {
	return x
}

func (x *Instance) Exports() modules.Exports {
	return modules.Exports{Default: x}
}

func (x *Instance) Setup(runContextBytes string) error {
	runContext, err := Serialized[*stroppy.StepContext](runContextBytes).Unmarshal()
	if err != nil {
		return err
	}

	x.logger.Debug(
		"Setup",
		zap.Uint64("seed", runContext.GetConfig().GetSeed()),
	)
	// TODO: think about cancel
	drv, _, err := driver.ConnectToPlugin(
		runContext.GetConfig(),
		x.logger,
	)
	if err != nil {
		return err
	}

	err = drv.Initialize(x.vu.Context(), runContext)
	if err != nil {
		return err
	}

	runPtr = newRuntimeContext(
		drv,
		x.logger,
		runContext,
	)

	return nil
}

//goland:noinspection t
func (x *Instance) GenerateQueue() (string, error) {
	stepQueries := make([]*stroppy.DriverQuery, 0)

	for _, queryDesc := range runPtr.runContext.GetStep().GetQueries() {
		queries, err := runPtr.driver.BuildQueries(
			x.vu.Context(),
			&stroppy.BuildQueriesContext{
				Context: runPtr.runContext,
				Query:   queryDesc,
			},
		)
		if err != nil {
			return "", err
		}

		stepQueries = append(stepQueries, queries.GetQueries()...)
	}

	return MarshalSerialized(&stroppy.DriverQueriesList{Queries: stepQueries})
}

func (x *Instance) RunQuery(queryData string) error {
	query, err := Serialized[*stroppy.DriverQuery](queryData).Unmarshal()
	if err != nil {
		return err
	}

	runPtr.logger.Debug(
		"RunQuery",
		zap.Any("query", query),
	)

	return runPtr.driver.RunQuery(
		x.vu.Context(),
		query,
	)
}

var ErrDriverIsNil = errors.New("driver is nil")

func (x *Instance) Teardown() error {
	if runPtr.driver == nil {
		return ErrDriverIsNil
	}

	return runPtr.driver.Teardown(x.vu.Context())
}
