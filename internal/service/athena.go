package service

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
)

// the entire purpose of this file is to add the new paritions after the cleaned up match json files are uploaded to S3
// this way we can avoid re-running a Glue crawler and having to wait for it to finish
// this probably shouldn't be run concurrently.
func repairGlueTable(ctx context.Context, client *athena.Client, tableName, glueDatabase, s3OutputPath string) error {
	// MSCK REPAIR TABLE for Glue table
	query := fmt.Sprintf("MSCK REPAIR TABLE `%s`.`%s`", glueDatabase, tableName)

	input := &athena.StartQueryExecutionInput{
		QueryString: aws.String(query),
		QueryExecutionContext: &types.QueryExecutionContext{
			Database: aws.String(glueDatabase),
			Catalog:  aws.String("AwsDataCatalog"), // Glue Data Catalog
		},
		ResultConfiguration: &types.ResultConfiguration{
			OutputLocation: aws.String(s3OutputPath),
		},
	}

	result, err := client.StartQueryExecution(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to start query: %w", err)
	}

	fmt.Printf("Query execution ID: %s\n", *result.QueryExecutionId)

	// Wait for completion
	return waitForQuery(ctx, client, *result.QueryExecutionId)
}

func waitForQuery(ctx context.Context, client *athena.Client, queryID string) error {
	for {
		output, err := client.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
			QueryExecutionId: aws.String(queryID),
		})
		if err != nil {
			return err
		}

		status := output.QueryExecution.Status.State

		switch status {
		case types.QueryExecutionStateSucceeded:
			fmt.Println("MSCK REPAIR TABLE completed successfully")
			if output.QueryExecution.Statistics != nil {
				fmt.Printf("Partitions added/updated\n")
			}
			return nil
		case types.QueryExecutionStateFailed:
			return fmt.Errorf("query failed: %s", *output.QueryExecution.Status.StateChangeReason)
		case types.QueryExecutionStateCancelled:
			return fmt.Errorf("query was cancelled")
		default:
			fmt.Printf("Query status: %s\n", status)
		}

		time.Sleep(2 * time.Second)
	}
}
