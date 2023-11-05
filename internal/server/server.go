package server

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/labstack/echo/v4"
	"github.com/wolfeidau/sfn-api-powered-workflows/internal/api/athena_workflow"
)

func Setup(awscfg aws.Config, e *echo.Echo) error {

	athenaSvc := athena.NewFromConfig(awscfg)

	srv := NewAthenaWorkflow(athenaSvc)

	athena_workflow.RegisterHandlers(e, srv)

	return nil
}

type Server struct {
	athenaSvc *athena.Client
}

func NewAthenaWorkflow(athenaSvc *athena.Client) *Server {
	return &Server{athenaSvc: athenaSvc}
}

// (POST /athena/run_query_template)
func (s *Server) RunAthenaQueryTemplate(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}

// (POST /athena/run_s3_query_template)
func (s *Server) RunS3AthenaQueryTemplate(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}
