package terraformrunner

import (
	"strings"
	"testing"
)

func Test_terraformRunner(t *testing.T) {
	terraformInput := `terraform {
	required_providers {
		docker = {
		source = "kreuzwerker/docker"
		}
	}
	}

	provider "docker" {}

	resource "docker_image" "nginx" {
	name         = "nginx:latest"
	keep_locally = false
	}

	resource "docker_container" "nginx" {
	image = docker_image.nginx.latest
	name  = "tutorial"
	ports {
		internal = 80
		external = 8000
	}
	}
	`
	planOutput, _ := TerraformRunner(terraformInput)

	planGenString := "Terraform used the selected providers to generate the following execution"
	if !strings.Contains(planOutput, planGenString) {
		t.Error("plan not generated")
	}

	planProperties := []string{`resource "docker_container" "nginx"`, `tutorial`, `internal = 80`, `external = 8000`}
	for _, element := range planProperties {
		if !strings.Contains(planOutput, element) {
			t.Error("plan does not contain detail: " + element)
		}
	}

}
