# sfn-api-powered-workflows

This project illustrates how to architect a serverless workflow using AWS Step Functions and APIs powered by AWS APIGW and Lambda.

The key goals of this project are:
* To demonstrate how to use AWS Step Functions to orchestrate serverless workflows
* To demonstrate how to use AWS APIGW and Lambda to create APIs that can be used to power AWS Step Functions workflows
* To demonstrate how openapi can be used to validate the API requests and responses from Tasks within a Step Functions workflow.

This will result in an architecture where APIs can be developed, versioned, deployed, and tested independently of the Step Functions workflows.

One of the key benefits of this approach is that we get to leverage openapi for documentation and validation of the API requests and responses.
