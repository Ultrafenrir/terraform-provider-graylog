//go:build acceptance

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOpenSearchSnapshotRepository_fs(t *testing.T) {
	if os.Getenv("ENABLE_OS_SNAPSHOT_ACC") == "" {
		t.Skip("OpenSearch snapshot repository acc test disabled; set ENABLE_OS_SNAPSHOT_ACC=1 to enable")
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWithOS() + `
resource "graylog_opensearch_snapshot_repository" "fs" {
  name = "tf-fs"
  type = "fs"
  fs_settings {
    location = "/usr/share/opensearch/snapshots"
    compress = true
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_opensearch_snapshot_repository.fs", "name", "tf-fs"),
					resource.TestCheckResourceAttr("graylog_opensearch_snapshot_repository.fs", "type", "fs"),
				),
			},
			{
				// Update some knob to verify idempotent PUT
				Config: testAccProviderConfigWithOS() + `
resource "graylog_opensearch_snapshot_repository" "fs" {
  name = "tf-fs"
  type = "fs"
  fs_settings {
    location                   = "/usr/share/opensearch/snapshots"
    max_snapshot_bytes_per_sec = "100mb"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_opensearch_snapshot_repository.fs", "type", "fs"),
				),
			},
		},
	})
}

func TestAccOpenSearchSnapshotRepository_s3(t *testing.T) {
	if os.Getenv("ENABLE_OS_SNAPSHOT_ACC") == "" {
		t.Skip("OpenSearch snapshot repository acc test disabled; set ENABLE_OS_SNAPSHOT_ACC=1 to enable")
	}
	if os.Getenv("ENABLE_OS_S3_ACC") == "" {
		t.Skip("S3 repository test disabled; set ENABLE_OS_S3_ACC=1 to enable (requires OpenSearch s3 plugin)")
	}
	// MinIO is expected to be available from docker-compose
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWithOS() + `
resource "graylog_opensearch_snapshot_repository" "s3" {
  name = "tf-s3"
  type = "s3"
  s3_settings {
    bucket            = "tf-snapshots"
    region            = "us-east-1"
    endpoint          = "http://minio:9000"
    protocol          = "http"
    path_style_access = true
    access_key        = "minioadmin"
    secret_key        = "minioadmin"
    base_path         = "acc"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_opensearch_snapshot_repository.s3", "name", "tf-s3"),
					resource.TestCheckResourceAttr("graylog_opensearch_snapshot_repository.s3", "type", "s3"),
				),
			},
		},
	})
}

// testAccProviderConfigWithOS extends testAccProviderConfig with OpenSearch URL
func testAccProviderConfigWithOS() string {
	url := os.Getenv("URL")
	if url == "" {
		url = "http://127.0.0.1:9000/api"
	}
	token := os.Getenv("TOKEN")
	osURL := os.Getenv("OPENSEARCH_URL")
	if osURL == "" {
		osURL = "http://127.0.0.1:9200"
	}
	return `
provider "graylog" {
  url   = "` + url + `"
  token = "` + token + `"
  opensearch_url = "` + osURL + `"
}
`
}

func TestAccOpenSearchSnapshotRepository_genericSettings_fs(t *testing.T) {
	if os.Getenv("ENABLE_OS_SNAPSHOT_ACC") == "" {
		t.Skip("OpenSearch snapshot repository acc test disabled; set ENABLE_OS_SNAPSHOT_ACC=1 to enable")
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWithOS() + `
resource "graylog_opensearch_snapshot_repository" "fs_generic" {
  name = "tf-fs-gen"
  type = "fs"
  settings = {
    location = "/usr/share/opensearch/snapshots"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_opensearch_snapshot_repository.fs_generic", "name", "tf-fs-gen"),
					resource.TestCheckResourceAttr("graylog_opensearch_snapshot_repository.fs_generic", "type", "fs"),
				),
			},
		},
	})
}
