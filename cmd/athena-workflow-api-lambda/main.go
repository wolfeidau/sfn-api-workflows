package main

import (
	"context"
	"io"

	"github.com/alecthomas/kong"
	"github.com/apex/gateway"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/labstack/echo/v4"
	echolog "github.com/labstack/gommon/log"
	"github.com/rs/zerolog/log"
	"github.com/wolfeidau/lambda-go-extras/standard"
	"github.com/wolfeidau/sfn-api-powered-workflows/internal/flags"
	"github.com/wolfeidau/sfn-api-powered-workflows/internal/server"
)

var (
	version = "dev"
	cfg     = new(flags.API)
)

func main() {
	kong.Parse(cfg,
		kong.Vars{"version": version}, // bind a var for version
	)

	awscfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load aws configuration")
	}

	e := echo.New()

	// shut down all the default output of echo
	e.Logger.SetOutput(io.Discard)
	e.Logger.SetLevel(echolog.OFF)

	gw := gateway.NewGateway(e)

	err = server.Setup(awscfg, e)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup server routes")
	}

	standard.Default(gw)
}
