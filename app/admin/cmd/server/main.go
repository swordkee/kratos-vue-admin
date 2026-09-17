package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"

	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
	"github.com/go-kratos/kratos/v3/transport/http"
	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flag conf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func newApp(_ log.Logger, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		// kratos v3 内部日志统一走标准库 slog；业务层日志仍用内嵌的 v2 兼容 log.Logger
		kratos.Logger(slog.New(slog.NewTextHandler(os.Stdout, nil))),
		kratos.Server(
			// gs,
			hs,
		),
	)
}

func newLogxLogger() *logx.Logger {
	slogLogger := slog.New(logx.BuildHandler(logx.HandlerConfig{
		Level:       "info",
		OutputPaths: "stdout",
	})).With(
		slog.String("service.name", Name),
		slog.String("service.version", Version),
		slog.String("host", id),
	)
	logx.SetDefault(slogLogger)
	return logx.NewLogger(slogLogger, logx.WithDesensitize())
}

func main() {
	flag.Parse()
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
	)
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Auth, bc.Casbin, bc.Oss, logger, newLogxLogger(), bc.Data.Redis)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
