package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	volumeTypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

func terraformRunner() {
	tf := []byte("terraform {\n\trequired_providers {\n\t\tdocker = {\n\t\tsource = \"kreuzwerker/docker\"\n\t\t}\n\t}\n\t}\n\n\tprovider \"docker\" {}\n\n\tresource \"docker_image\" \"nginx\" {\n\tname         = \"nginx:latest\"\n\tkeep_locally = false\n\t}\n\n\tresource \"docker_container\" \"nginx\" {\n\timage = docker_image.nginx.latest\n\tname  = \"tutorial\"\n\tports {\n\t\tinternal = 80\n\t\texternal = 8000\n\t}\n\t}\\n")

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
	options := cpConfig{
		followLink: false,
		copyUIDGID: false,
		quiet:      false,
		sourcePath: "terraformTestFiles/docker-nginx.tf",
		destPath:   "app",
		container:  resp.ID,
	}
	err = copyToContainer(ctx, cli, options)
	if err != nil {
		panic(err)
	}

	// sudo docker exec -it terraform terraform  init
	execResp, err := cli.ContainerExecCreate(ctx, resp.ID, types.ExecConfig{
		AttachStdin:  false,
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"terraform", "init"},
	})
	if err != nil {
		panic(err)
	}

	attach, err := cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		panic(err)
	}

	// make sure terraform init finishes before moving on
	c := attach.Conn

	one := make([]byte, 1)
	_, err = c.Read(one)

	for err != io.EOF {
		fmt.Println("Waiting for terraform init to finish")
		_, err = c.Read(one)
	}

	// logging
	defer attach.Close()
	go io.Copy(os.Stdout, attach.Reader)

	// sudo docker exec -it terraform sh -c "terraform plan -no-color > output.txt"
	execResp2, err := cli.ContainerExecCreate(ctx, resp.ID, types.ExecConfig{
		AttachStdin:  false,
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"/bin/sh", "-c", "terraform plan -no-color | tee output.txt"},
	})
	if err != nil {
		panic(err)
	}

	attach2, err := cli.ContainerExecAttach(ctx, execResp2.ID, types.ExecStartCheck{})
	if err != nil {
		panic(err)
	}

	//make sure terraform plan finishes before moving on
	c = attach2.Conn

	one = make([]byte, 1)
	_, err = c.Read(one)

	for err != io.EOF {
		fmt.Println("Waiting for terraform plan to finish")
		_, err = c.Read(one)
	}

	// log
	defer attach2.Close()
	go io.Copy(os.Stdout, attach2.Reader)

	// sudo docker cp terraform:/app/output.txt
	contents, _, err := cli.CopyFromContainer(ctx, resp.ID, "/app/output.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println(contents)

	// sudo docker container stop terraform
	err = cli.ContainerKill(ctx, resp.ID, "KILL")
	if err != nil {
		fmt.Printf("Unable to stop container %s", resp.ID)
		panic(err)
	}

	// sudo docker container prune
	_, err = cli.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		fmt.Printf("Unable to prune containers")
		panic(err)
	}

	// sudo docker volume prune
	_, err = cli.VolumesPrune(ctx, filters.Args{})
	if err != nil {
		fmt.Printf("Unable to prune volumes")
		panic(err)
	}

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
