package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	volumeTypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	// "github.com/docker/docker/pkg/stdcopy"
)

func terraformRunner() {
	// tf :=
	// 	`terraform {
	// required_providers {
	// 	docker = {
	// 	source = "kreuzwerker/docker"
	// 	}
	// }
	// }

	// provider "docker" {}

	// resource "docker_image" "nginx" {
	// name         = "nginx:latest"
	// keep_locally = false
	// }

	// resource "docker_container" "nginx" {
	// image = docker_image.nginx.latest
	// name  = "tutorial"
	// ports {
	// 	internal = 80
	// 	external = 8000
	// }
	// }`

	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// sudo docker volume create tfplan-test
	vol, err := cli.VolumeCreate(ctx, volumeTypes.VolumeCreateBody{Name: "tfplan-test"})

	reader, err := cli.ImagePull(ctx, "hashicorp/terraform:light", types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, reader)

	// sudo docker run -d -it --name terraform --entrypoint "/usr/bin/tail" -v /var/run/docker.sock:/var/run/docker.sock -v tfplan-test:/app -w /app hashicorp/terraform:light tail -f /dev/null
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      "hashicorp/terraform:light",
		Entrypoint: []string{"/usr/bin/tail"},
		WorkingDir: "/app",
		Cmd:        []string{"tail", "-f", "/dev/null"},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			mount.Mount{
				Source:   vol.Name,
				Target:   "/app",
				Type:     "volume",
				ReadOnly: false,
			},
		},
		Binds: []string{
			"/var/run/docker.sock:/var/run/docker.sock",
		},
	}, &network.NetworkingConfig{}, nil, "terraform")
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	// sudo docker cp main.tf terraform:/app
	// sudo docker exec -it terraform terraform  init
	// sudo docker exec -it terraform sh -c "terraform plan -no-color > output.txt"
	// sudo docker cp terraform:/app/output.txt .
	// sudo docker container stop terraform
	// sudo docker container rm terraform
	// sudo docker volume prune
	fmt.Println(resp.ID)

	// statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	// select {
	// case err := <-errCh:
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// case <-statusCh:
	// }

	// out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	// if err != nil {
	// 	panic(err)
	// }

	// stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}
