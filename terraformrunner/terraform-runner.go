package terraformrunner

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudcapable/dockerutils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	volumeTypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// TerraformRunner is a function
func TerraformRunner(terraformInput string) (string, error) {
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	evalSessionUUID := uuid.New().String()

	// sudo docker volume create tfplan-test
	vol, err := cli.VolumeCreate(ctx, volumeTypes.VolumeCreateBody{Name: evalSessionUUID})

	reader, err := cli.ImagePull(ctx, "hashicorp/terraform:light", types.ImagePullOptions{})
	if err != nil {
		return "", err
	}
	io.Copy(os.Stdout, reader)

	// sudo docker run -d -it --name terraform --entrypoint "/usr/bin/tail" -v /var/run/docker.sock:/var/run/docker.sock -v tfplan-test:/app -w /app hashicorp/terraform:light tail -f /dev/null
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        "hashicorp/terraform:light",
		Entrypoint:   []string{"/usr/bin/tail"},
		WorkingDir:   "/app",
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{},
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
	}, &network.NetworkingConfig{}, nil, evalSessionUUID)
	if err != nil {
		return "", err
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	// sudo docker cp main.tf terraform:/app
	options := dockerutils.CpConfig{
		CopyUIDGID: false,
		Quiet:      false,
		DestPath:   "app",
		Container:  resp.ID,
	}

	err = dockerutils.CopyStringToContainer(ctx, cli, options, terraformInput)
	if err != nil {
		return "", err
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
		return "", err
	}

	attach, err := cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
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
		return "", err
	}

	attach2, err := cli.ContainerExecAttach(ctx, execResp2.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
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
	contents, _, err := cli.CopyFromContainer(ctx, resp.ID, "app/output.txt")
	if err != nil {
		return "", err
	}
	buf := new(strings.Builder)
	_, err = io.Copy(buf, contents)
	fmt.Println(buf.String())

	go cleanUp(ctx, cli, resp, err)

	return buf.String(), nil
}

func cleanUp(ctx context.Context, cli *client.Client, resp container.ContainerCreateCreatedBody, err error) {
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
}
