package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ResourceInfo struct {
	ServiceName  string            `json:"service"`
	ResourceID   string            `json:"resource_id"`
	ResourceType string            `json:"resource_type"`
	Tags        map[string]string `json:"tags"`
	MissingTags []string          `json:"missing_tags"`
}

type AuditReport struct {
	Resources []ResourceInfo `json:"resources"`
}

type Auditor struct {
	ec2Client    *ec2.Client
	s3Client     *s3.Client
	rdsClient    *rds.Client
	lambdaClient *lambda.Client
}

func NewAuditor() *Auditor {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("unable to load SDK config: %v", err))
	}

	return &Auditor{
		ec2Client:    ec2.NewFromConfig(cfg),
		s3Client:     s3.NewFromConfig(cfg),
		rdsClient:    rds.NewFromConfig(cfg),
		lambdaClient: lambda.NewFromConfig(cfg),
	}
}

func (a *Auditor) AuditResources(ctx context.Context, services, requiredTags []string) (*AuditReport, error) {
	report := &AuditReport{}
	var wg sync.WaitGroup
	resourceChan := make(chan ResourceInfo)
	errorChan := make(chan error)
	done := make(chan bool)

	for _, service := range services {
		wg.Add(1)
		go func(svc string) {
			defer wg.Done()
			switch strings.ToLower(svc) {
			case "ec2":
				a.auditEC2(ctx, requiredTags, resourceChan, errorChan)
			case "s3":
				a.auditS3(ctx, requiredTags, resourceChan, errorChan)
			case "rds":
				a.auditRDS(ctx, requiredTags, resourceChan, errorChan)
			case "lambda":
				a.auditLambda(ctx, requiredTags, resourceChan, errorChan)
			}
		}(service)
	}

	// Collector goroutine
	go func() {
		wg.Wait()
		done <- true
	}()

	// Collect results
	for {
		select {
		case resource := <-resourceChan:
			report.Resources = append(report.Resources, resource)
		case err := <-errorChan:
			return nil, err
		case <-done:
			return report, nil
		}
	}
}

func (r *AuditReport) OutputJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

func (r *AuditReport) OutputText(w io.Writer) error {
	fmt.Fprintln(w, "AWS Resource Tag Audit Report")
	fmt.Fprintln(w, "==========================")
	
	for _, resource := range r.Resources {
		fmt.Fprintf(w, "\nService: %s\n", resource.ServiceName)
		fmt.Fprintf(w, "Resource ID: %s\n", resource.ResourceID)
		fmt.Fprintf(w, "Resource Type: %s\n", resource.ResourceType)
		fmt.Fprintf(w, "Tags: %v\n", resource.Tags)
		if len(resource.MissingTags) > 0 {
			fmt.Fprintf(w, "Missing Tags: %v\n", resource.MissingTags)
		}
		fmt.Fprintln(w, "--------------------------")
	}
	return nil
}

func (r *AuditReport) OutputCSV(w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{"Service", "Resource ID", "Resource Type", "Tags", "Missing Tags"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("error writing CSV header: %w", err)
	}

	// Write data rows
	for _, resource := range r.Resources {
		// Convert tags map to string
		tagsStr := make([]string, 0, len(resource.Tags))
		for k, v := range resource.Tags {
			tagsStr = append(tagsStr, fmt.Sprintf("%s:%s", k, v))
		}

		row := []string{
			resource.ServiceName,
			resource.ResourceID,
			resource.ResourceType,
			strings.Join(tagsStr, "; "),
			strings.Join(resource.MissingTags, "; "),
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("error writing CSV row: %w", err)
		}
	}

	return nil
}

func (a *Auditor) auditEC2(ctx context.Context, requiredTags []string, resourceChan chan<- ResourceInfo, errorChan chan<- error) {
	input := &ec2.DescribeInstancesInput{}
	result, err := a.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		errorChan <- fmt.Errorf("failed to describe EC2 instances: %w", err)
		return
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			tags := make(map[string]string)
			missingTags := make([]string, 0)

			for _, tag := range instance.Tags {
				tags[*tag.Key] = *tag.Value
			}

			for _, required := range requiredTags {
				if _, exists := tags[required]; !exists {
					missingTags = append(missingTags, required)
				}
			}

			resourceChan <- ResourceInfo{
				ServiceName:  "EC2",
				ResourceID:   *instance.InstanceId,
				ResourceType: "Instance",
				Tags:        tags,
				MissingTags: missingTags,
			}
		}
	}
}

func (a *Auditor) auditS3(ctx context.Context, requiredTags []string, resourceChan chan<- ResourceInfo, errorChan chan<- error) {
	input := &s3.ListBucketsInput{}
	result, err := a.s3Client.ListBuckets(ctx, input)
	if err != nil {
		errorChan <- fmt.Errorf("failed to list S3 buckets: %w", err)
		return
	}

	for _, bucket := range result.Buckets {
		tagsInput := &s3.GetBucketTaggingInput{
			Bucket: bucket.Name,
		}

		tags := make(map[string]string)
		missingTags := make([]string, 0)

		tagsOutput, err := a.s3Client.GetBucketTagging(ctx, tagsInput)
		if err == nil {
			for _, tag := range tagsOutput.TagSet {
				tags[*tag.Key] = *tag.Value
			}
		}

		for _, required := range requiredTags {
			if _, exists := tags[required]; !exists {
				missingTags = append(missingTags, required)
			}
		}

		resourceChan <- ResourceInfo{
			ServiceName:  "S3",
			ResourceID:   *bucket.Name,
			ResourceType: "Bucket",
			Tags:        tags,
			MissingTags: missingTags,
		}
	}
}

func (a *Auditor) auditRDS(ctx context.Context, requiredTags []string, resourceChan chan<- ResourceInfo, errorChan chan<- error) {
	input := &rds.DescribeDBInstancesInput{}
	result, err := a.rdsClient.DescribeDBInstances(ctx, input)
	if err != nil {
		errorChan <- fmt.Errorf("failed to describe RDS instances: %w", err)
		return
	}

	for _, instance := range result.DBInstances {
		tags := make(map[string]string)
		missingTags := make([]string, 0)

		for _, tag := range instance.TagList {
			tags[*tag.Key] = *tag.Value
		}

		for _, required := range requiredTags {
			if _, exists := tags[required]; !exists {
				missingTags = append(missingTags, required)
			}
		}

		resourceChan <- ResourceInfo{
			ServiceName:  "RDS",
			ResourceID:   *instance.DBInstanceIdentifier,
			ResourceType: "DBInstance",
			Tags:        tags,
			MissingTags: missingTags,
		}
	}
}

func (a *Auditor) auditLambda(ctx context.Context, requiredTags []string, resourceChan chan<- ResourceInfo, errorChan chan<- error) {
	input := &lambda.ListFunctionsInput{}
	result, err := a.lambdaClient.ListFunctions(ctx, input)
	if err != nil {
		errorChan <- fmt.Errorf("failed to list Lambda functions: %w", err)
		return
	}

	for _, function := range result.Functions {
		tagsInput := &lambda.ListTagsInput{
			Resource: function.FunctionArn,
		}

		tagsOutput, err := a.lambdaClient.ListTags(ctx, tagsInput)
		if err != nil {
			errorChan <- fmt.Errorf("failed to get tags for Lambda function %s: %w", *function.FunctionName, err)
			continue
		}

		tags := make(map[string]string)
		missingTags := make([]string, 0)

		for key, value := range tagsOutput.Tags {
			tags[key] = value
		}

		for _, required := range requiredTags {
			if _, exists := tags[required]; !exists {
				missingTags = append(missingTags, required)
			}
		}

		resourceChan <- ResourceInfo{
			ServiceName:  "Lambda",
			ResourceID:   *function.FunctionName,
			ResourceType: "Function",
			Tags:        tags,
			MissingTags: missingTags,
		}
	}
}
