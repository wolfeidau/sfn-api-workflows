package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/wolfeidau/s3iofs"
	"github.com/wolfeidau/sfn-api-powered-workflows/internal/api/athena_workflow"
	"github.com/wolfeidau/sfn-api-powered-workflows/internal/flags"
)

const (
	defaultTimeoutSeconds int64 = 30
)

// Setup configures the API server by registering the Athena workflow
// handlers and initializing the Athena service client.
func Setup(cfg flags.API, awscfg aws.Config, e *echo.Echo) error {

	athenaSvc := athena.NewFromConfig(awscfg)
	s3Svc := s3.NewFromConfig(awscfg)

	srv := NewAthenaWorkflow(cfg, athenaSvc, s3Svc)

	athena_workflow.RegisterHandlers(e, srv)

	return nil
}

type Server struct {
	cfg       flags.API
	athenaSvc *athena.Client
	s3Svc     *s3.Client
}

// NewAthenaWorkflow creates a new Server instance with the provided configuration and Athena service client.
func NewAthenaWorkflow(cfg flags.API, athenaSvc *athena.Client, s3Svc *s3.Client) *Server {
	return &Server{
		cfg:       cfg,
		athenaSvc: athenaSvc,
		s3Svc:     s3Svc,
	}
}

// (POST /athena/run_query_template).
func (s *Server) RunAthenaQueryTemplate(c echo.Context) error {

	ctx := c.Request().Context()

	runAthenaQuery := new(athena_workflow.RunAthenaQueryTemplateRequest)

	err := c.Bind(runAthenaQuery)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to parse request")

		return errorResponse(c, http.StatusBadRequest, "failed to parse request")
	}

	athenaQuery, err := executeTextTemplate(runAthenaQuery.TemplateQuery, runAthenaQuery.TemplateData)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to execute template")

		return errorResponse(c, http.StatusBadRequest, "failed to execute template")
	}

	var parameters []string
	// if the query has parameters, then use them
	if runAthenaQuery.Parameters != nil {
		parameters = *runAthenaQuery.Parameters
	}

	queryResult, err := executeAthenaQuery(ctx, s.athenaSvc, athenaQuery, s.cfg.AthenaCatalog, s.cfg.AthenaDatabase, s.cfg.AthenaWorkgroup, parameters, runAthenaQuery.WaitForCompletion)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to run query")

		return errorResponse(c, http.StatusInternalServerError, "failed to run query")
	}

	return c.JSON(http.StatusOK, queryResult)
}

// (POST /athena/run_s3_query_template).
func (s *Server) RunS3AthenaQueryTemplate(c echo.Context) error {
	ctx := c.Request().Context()

	runAthenaQuery := new(athena_workflow.RunS3AthenaQueryTemplateRequest)

	err := c.Bind(runAthenaQuery)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to parse request")

		return errorResponse(c, http.StatusBadRequest, "failed to parse request")
	}

	templatesDir := s3iofs.NewWithClient(s.cfg.QueryTemplatesBucket, s.s3Svc)

	athenaQuery, err := executeTextTemplateWithFS(templatesDir, runAthenaQuery.TemplateName, runAthenaQuery.TemplateData, runAthenaQuery.TemplateParsePatterns...)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to execute template")

		return errorResponse(c, http.StatusBadRequest, "failed to execute template")
	}

	var parameters []string
	// if the query has parameters, then use them
	if runAthenaQuery.Parameters != nil {
		parameters = *runAthenaQuery.Parameters
	}

	queryResult, err := executeAthenaQuery(ctx, s.athenaSvc, athenaQuery, s.cfg.AthenaCatalog, s.cfg.AthenaDatabase, s.cfg.AthenaWorkgroup, parameters, runAthenaQuery.WaitForCompletion)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to run query")

		return errorResponse(c, http.StatusInternalServerError, "failed to run query")
	}

	return c.JSON(http.StatusOK, queryResult)
}

// executeTextTemplateWithFS executes a text template loaded from the provided fs.FS
// interface. It parses the template patterns from the fs.FS, executes the named
// template with the provided data, and returns the result string or any errors.
func executeTextTemplateWithFS(fsys fs.FS, name string, data interface{}, patterns ...string) (string, error) {
	tmpl, err := template.New("query").ParseFS(fsys, patterns...)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	buf := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(buf, name, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// executeTextTemplate executes a Go text/template with the given queryTemplate and data.
// It returns the executed template string, or an error if template parsing or execution fails.
func executeTextTemplate(queryTemplate string, data interface{}) (string, error) {
	tmpl, err := template.New("query").Parse(queryTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	buf := new(bytes.Buffer)

	err = tmpl.Execute(buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func errorResponse(c echo.Context, code int, msg string) error {
	return c.JSON(code,
		athena_workflow.ErrorResponse{
			Message: msg},
	)
}

// executeAthenaQuery starts an Athena query execution and returns the query execution ID.
func executeAthenaQuery(ctx context.Context, athenaClient *athena.Client, query, catalogue, database, workgroup string, parameters []string, waitForCompletion bool) (*athena_workflow.RunAthenaQueryTemplateResponse, error) {
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

	if !waitForCompletion {
		return &athena_workflow.RunAthenaQueryTemplateResponse{
			QueryExecutionId: aws.ToString(queryResult.QueryExecutionId),
		}, nil
	}

	deadline := time.Now().Add(time.Duration(defaultTimeoutSeconds) * time.Second)

	return waitForQuery(ctx, athenaClient, aws.ToString(queryResult.QueryExecutionId), deadline)
}

// waitForQuery polls Athena for the status of a query execution until it completes or times out.
// It returns the final RunAthenaQueryTemplateResponse or an error if the query fails or times out.
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

			if queryExecution.QueryExecution.Status.State != types.QueryExecutionStateSucceeded {
				log.Ctx(ctx).Error().Fields(queryExecution).Msg("failed to run query, result was not successful")

				return nil, fmt.Errorf("query failed with status: %s", queryExecution.QueryExecution.Status.State)
			}

			return &athena_workflow.RunAthenaQueryTemplateResponse{
				QueryExecutionId:    queryExecutionId,
				QueryExecutionState: aws.String(string(queryExecution.QueryExecution.Status.State)),
				ResultPath:          queryExecution.QueryExecution.ResultConfiguration.OutputLocation,
			}, nil
		}

		time.Sleep(5 * time.Second)
	}

	return nil, errors.New("query timed out")
}
