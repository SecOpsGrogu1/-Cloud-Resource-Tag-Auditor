# Cloud-Resource-Tag-Auditor

A CLI tool to scan AWS resources across services (e.g., EC2, S3, RDS, Lambda) and generate a report of resources missing required tags or with inconsistent tagging.

## Features

- Scans multiple AWS services (EC2, S3, RDS, Lambda)
- Identifies resources missing required tags
- Supports custom tag requirements
- Outputs reports in both text, JSON and CSV formats
- Concurrent scanning for faster results

## Prerequisites

- Go 1.21 or later
- AWS credentials configured (either through AWS CLI or environment variables)
- Appropriate AWS IAM permissions to read resources and their tags

## Installation

```bash
go install github.com/SecOpsGrogu1/-Cloud-Resource-Tag-Auditor@latest
```

Or clone and build manually:

```bash
git clone https://github.com/SecOpsGrogu1/-Cloud-Resource-Tag-Auditor.git
cd cloud-resource-tag-auditor
go build
```

## Usage

Basic usage:

```bash
./tag-auditor audit -t environment,project,owner

# Specify services to audit
./tag-auditor audit -t environment,project,owner -s ec2,s3

# Output in JSON format
./tag-auditor audit -t environment,project,owner -o json

# Output in CSV format
./tag-auditor audit -t environment,project,owner -o csv
```

### Available Commands

- `audit`: Run the tag compliance audit

### Flags

- `-t, --required-tags`: List of required tags (comma-separated)
- `-s, --services`: AWS services to audit (default: "ec2,s3,rds,lambda")
- `-o, --output`: Output format (text/json/csv) (default: "text")

## Required AWS Permissions

The tool requires the following AWS permissions:
- ec2:DescribeInstances
- s3:ListBuckets
- s3:GetBucketTagging
- rds:DescribeDBInstances
- lambda:ListFunctions
- lambda:ListTags

## Example Output

Text format:
```
AWS Resource Tag Audit Report
==========================
Service: EC2
Resource ID: i-1234567890abcdef0
Resource Type: Instance
Tags: map[Name:webserver environment:production]
Missing Tags: [project owner]
--------------------------
```

CSV format:
```csv
Service,Resource ID,Resource Type,Tags,Missing Tags
EC2,i-1234567890abcdef0,Instance,"Name:webserver; environment:production","project; owner"
```

JSON format:
```json
{
  "resources": [
    {
      "service": "EC2",
      "resource_id": "i-1234567890abcdef0",
      "resource_type": "Instance",
      "tags": {
        "Name": "webserver",
        "environment": "production"
      },
      "missing_tags": ["project", "owner"]
    }
  ]
}
```

## License

MIT License
