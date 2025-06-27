package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create an AWS S3 bucket for media storage
		bucket, err := s3.NewBucket(ctx, "transgo-media-backend", &s3.BucketArgs{
			Acl: pulumi.String("private"),
			Versioning: &s3.BucketVersioningArgs{
				Enabled: pulumi.Bool(true),
			},
			Tags: pulumi.StringMap{
				"Project":     pulumi.String("transgo"),
				"Environment": pulumi.String("shared"),
				"ManagedBy":   pulumi.String("pulumi"),
			},
		})
		if err != nil {
			return err
		}

		// Export the bucket name and ARN
		ctx.Export("bucketName", bucket.ID())
		ctx.Export("bucketArn", bucket.Arn)
		return nil
	})
}
