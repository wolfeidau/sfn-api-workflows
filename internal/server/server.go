package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/wolfeidau/sfn-api-powered-workflows/internal/api/athena_workflow"
	"github.com/wolfeidau/sfn-api-powered-workflows/internal/flags"
)

const (
	defaultTimeoutSeconds int64 = 30
)

func Setup(cfg flags.API, awscfg aws.Config, e *echo.Echo) error {

	athenaSvc := athena.NewFromConfig(awscfg)

	srv := NewAthenaWorkflow(cfg, athenaSvc)

	athena_workflow.RegisterHandlers(e, srv)

	return nil
}

type Server struct {
	cfg       flags.API
	athenaSvc *athena.Client
}

func NewAthenaWorkflow(cfg flags.API, athenaSvc *athena.Client) *Server {
	return &Server{
		cfg:       cfg,
		athenaSvc: athenaSvc,
	}
}

// (POST /athena/run_query_template)
func (s *Server) RunAthenaQueryTemplate(c echo.Context) error {

	ctx := c.Request().Context()

	runAthenaQuery := new(athena_workflow.RunAthenaQueryTemplateRequest)

	err := c.Bind(runAthenaQuery)
	if err != nil {
		return c.JSON(http.StatusInternalServerError,
			athena_workflow.ErrorResponse{
				Message: "failed to parse request"},
		)
	}

	deadline := time.Now().Add(time.Duration(defaultTimeoutSeconds) * time.Second)

	queryResult, err := executeAthenaQuerySync(ctx, s.athenaSvc, runAthenaQuery.Query, s.cfg.AthenaCatalog, s.cfg.AthenaDatabase, s.cfg.AthenaWorkgroup, runAthenaQuery.Parameters)
	if err != nil {
		return c.JSON(http.StatusInternalServerError,
			athena_workflow.ErrorResponse{
				Message: "failed to run query"},
		)
	}

	res, err := waitForQuery(ctx, s.athenaSvc, aws.ToString(queryResult.QueryExecutionId), deadline)
	if err != nil {
		return c.JSON(http.StatusInternalServerError,
			athena_workflow.ErrorResponse{
				Message: "failed to run query"},
		)
	}

	return c.JSON(http.StatusOK, res)
}

// (POST /athena/run_s3_query_template)
func (s *Server) RunS3AthenaQueryTemplate(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}

func executeAthenaQuerySync(ctx context.Context, athenaClient *athena.Client, query, catalogue, database, workgroup string, parameters []string) (*athena.StartQueryExecutionOutput, error) {
	queryResult, err := athenaClient.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString:         aws.String(query),
		ExecutionParameters: parameters,
		QueryExecutionContext: &types.QueryExecutionContext{
			Catalog:  aws.String(catalogue),
			Database: aws.String(database),
		},
		WorkGroup: aws.String(workgroup),
	})
	if err != nil {
		return nil, err
	}

	return queryResult, nil
}

func waitForQuery(ctx context.Context, athenaClient *athena.Client, queryExecutionId string, deadline time.Time) (*athena_workflow.RunAthenaQueryTemplateResponse, error) {
	for time.Now().Before(deadline) {
		queryExecution, err := athenaClient.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
			QueryExecutionId: aws.String(queryExecutionId),
		})

		if err != nil {
			return nil, err
		}

		switch queryExecution.QueryExecution.Status.State {
		case types.QueryExecutionStateRunning, types.QueryExecutionStateQueued:
			log.Ctx(ctx).Info().Str("QueryExecutionId", queryExecutionId).Msg("query still running")
		default:
			return &athena_workflow.RunAthenaQueryTemplateResponse{
				QueryExecutionId:    queryExecutionId,
				QueryExecutionState: string(queryExecution.QueryExecution.Status.State),
				ResultPath:          aws.ToString(queryExecution.QueryExecution.ResultConfiguration.OutputLocation),
			}, nil
		}

		time.Sleep(5 * time.Second)
	}

	return nil, errors.New("query timed out")
}
