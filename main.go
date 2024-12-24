package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "tag-auditor",
		Short: "AWS Resource Tag Auditor",
		Long: `A CLI tool to scan AWS resources across services and generate a report of resources 
missing required tags or with inconsistent tagging.`,
	}

	var requiredTags []string
	var outputFormat string
	var services []string

	var auditCmd = &cobra.Command{
		Use:   "audit",
		Short: "Audit AWS resources for tag compliance",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runAudit(services, requiredTags, outputFormat); err != nil {
				log.Fatalf("Error running audit: %v", err)
			}
		},
	}

	auditCmd.Flags().StringSliceVarP(&requiredTags, "required-tags", "t", []string{}, "List of required tags (comma-separated)")
	auditCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text/json/csv)")
	auditCmd.Flags().StringSliceVarP(&services, "services", "s", []string{"ec2", "s3", "rds", "lambda"}, "AWS services to audit (comma-separated)")
	auditCmd.MarkFlagRequired("required-tags")

	rootCmd.AddCommand(auditCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runAudit(services, requiredTags []string, outputFormat string) error {
	ctx := context.Background()
	auditor := NewAuditor()
	
	report, err := auditor.AuditResources(ctx, services, requiredTags)
	if err != nil {
		return fmt.Errorf("failed to audit resources: %w", err)
	}

	switch outputFormat {
	case "json":
		return report.OutputJSON(os.Stdout)
	case "csv":
		return report.OutputCSV(os.Stdout)
	case "text":
		return report.OutputText(os.Stdout)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}
