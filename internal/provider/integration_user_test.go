//go:build integration

package provider

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func TestIntegration_UserCRUD(t *testing.T) {
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	c := client.New(baseURL, token)

	username := "tf-itest-user"
	created, err := c.CreateUser(&client.User{
		Username: username,
		FullName: "TF ITest",
		Email:    "tf-itest@example.com",
		Roles:    []string{"Reader"},
		Password: "ChangeMe123!",
	})
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	defer func() { _ = c.DeleteUser(created.Username) }()

	got, err := c.GetUser(username)
	if err != nil {
		t.Fatalf("GetUser error: %v", err)
	}
	if got.Username != username {
		t.Fatalf("unexpected user: %+v", got)
	}

	// Update step is not compatible with Graylog v5 (endpoint expects ObjectId). For v5 do create/get/delete only.
	if c.APIVersion != client.APIV5 {
		// Update: disable and change full name
		_, err = c.UpdateUser(username, &client.User{
			FullName: "TF ITest Updated",
			Disabled: true,
			Roles:    got.Roles,
			Email:    got.Email,
			Timezone: got.Timezone,
		})
		if err != nil {
			t.Fatalf("UpdateUser error: %v", err)
		}
		after, err := c.GetUser(username)
		if err != nil {
			t.Fatalf("GetUser after update error: %v", err)
		}
		if !after.Disabled || after.FullName != "TF ITest Updated" {
			t.Fatalf("user not updated: %+v", after)
		}
	}

	if err := c.DeleteUser(username); err != nil {
		t.Fatalf("DeleteUser error: %v", err)
	}
}
